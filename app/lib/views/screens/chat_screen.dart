import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:glowby/views/screens/global_settings.dart';
import 'package:url_launcher/url_launcher_string.dart';
import 'package:glowby/views/widgets/tasks_view.dart';

import '../../models/ai.dart';
import '../dialogs/ai_settings_dialog.dart';
import 'magical_loading_view.dart';
import '../widgets/message.dart';
import '../widgets/new_message.dart';
import '../widgets/messages.dart';
import '../../services/openai_api.dart';
import '../../utils/text_to_speech.dart'; // Import the new TextToSpeech class
import '../dialogs/api_key_dialog.dart';
import '../../utils/timestamp.dart'; // Import the ApiKeyDialog widget
import 'package:async/async.dart';
import 'package:flutter/foundation.dart';

class ChatScreen extends StatefulWidget {
  final List<Map<String, Object>> _questions;
  final String _name;
  final bool _voice;
  final String? _selectedModel;
  final String? _selectedLanguage;
  final String? _systemPrompt;
  final bool? _allowEnterKey;
  final bool? _autonomousMode;
  final bool? _enableAi;
  final bool? _showAiSettings;
  final bool? _dnsgs;

  const ChatScreen(
      this._name,
      this._questions,
      this._voice,
      this._selectedModel,
      this._selectedLanguage,
      this._systemPrompt,
      this._allowEnterKey,
      this._autonomousMode,
      this._enableAi,
      this._showAiSettings,
      this._dnsgs,
      {super.key});

  @override
  ChatScreenState createState() => ChatScreenState();
}

class ChatScreenState extends State<ChatScreen> {
  var _autonomousMode = false;
  bool _loading = false;
  bool _voiceEnabled = true;
  void updateVoiceEnabled(bool value) {
    setState(() {
      _voiceEnabled = value;
    });
  }

  final TextToSpeech textToSpeech = TextToSpeech(); // Initialize TextToSpeech

  final List<Message> _messages = [];

  @override
  void initState() {
    super.initState();
    initializeVoiceEnabled();
    loadGlobalSettings();
    loadAPIKey();
  }

  void initializeVoiceEnabled() {
    _voiceEnabled = widget._voice;
  }

  void loadGlobalSettings() {
    GlobalSettings().loadDialogValues(
      widget._selectedModel,
      widget._selectedLanguage,
      widget._systemPrompt,
      widget._autonomousMode,
    );
  }

  void loadAPIKey() {
    OpenAiApi.loadOat().then((_) => setState(() {}));
  }

  // Refresh the UI state of the chat screen
  void refreshUI() {
    setState(() {});
  }

// Handle text-to-speech functionality independently
  void handleTextToSpeech() {
    if (widget._voice && _voiceEnabled) {
      try {
        if (_messages.isNotEmpty &&
            _messages[0].userId == '007' &&
            !_planImplementationInProgress) {
          textToSpeech.speakText(_messages[0].text,
              language: GlobalSettings().selectedLanguage);
        }
      } catch (e) {
        if (kDebugMode) {
          print('Error in text-to-speech: $e');
        }
      }
    }
  }

  void refresh() {
    handleTextToSpeech();
    refreshUI();
  }

  void _showApiKeyDialog() {
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return const ApiKeyDialog(); // Use the ApiKeyDialog widget
      },
    ).then(
      (value) => setState(() {}),
    );
  }

  void _showAiSettingsDialog() {
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return AiSettingsDialog(
          onVoiceEnabledChanged: updateVoiceEnabled,
        );
      },
    );
  }

  String extractPlanName(String response, String inputMessage) {
    RegExp inputMessagePattern = RegExp(
        r"(?:I want to|I'd like to|I'd love to|I wanna|let's|I desire to) (.+)",
        caseSensitive: false);
    RegExpMatch? inputMatch = inputMessagePattern.firstMatch(inputMessage);

    if (inputMatch != null && inputMatch.groupCount > 0) {
      return inputMatch.group(1)!;
    } else {
      RegExp planNamePattern = RegExp(
          r"Here's a (?:\d+-step|five-step) plan(?: to| for)?(?: break)?(?:ing)?(?: it)?(?: down)?(?: into)? ([^:]+):");
      RegExpMatch? match = planNamePattern.firstMatch(response);

      if (match != null && match.groupCount > 0) {
        return match.group(1)!;
      } else {
        return 'Unnamed Plan';
      }
    }
  }

  CancelableOperation<String>? _currentOperation;

  Future<List<String>> _generateTasks(String inputMessage) async {
    if (inputMessage != '') {
      _lastInputMessage = inputMessage;
    }

    if (_lastInputMessage == '') {
      return [];
    }

    setState(() {
      _loading = true;
    });

    List<String> tasks = [];

    try {
      _currentOperation = OpenAiApi.getResponseFromOpenAI(_lastInputMessage,
          customSystemPrompt:
              'You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. The steps should be concise.');

      String response = await _currentOperation!.value;

      if (response ==
          'Sorry, there was an error processing your request. Please try again later.') {
        _stopAutonomousMode();
        Future.delayed(const Duration(microseconds: 200), () {
          setState(() {
            _autonomousMode = false;
          });
        });

        textToSpeech.speakText(
            'Sorry, there was an error processing your request. Please try again later.');
        return [];
      }

      //print('response: $response');
      _planName = extractPlanName(response, _lastInputMessage);

      RegExp stepPattern =
          RegExp(r'(?:Step\s*\d+\s*:|^\d+\.)', multiLine: true);
      Iterable<RegExpMatch> matches = stepPattern.allMatches(response);

      int startIndex = 0;
      for (RegExpMatch match in matches) {
        int endIndex = match.start;
        if (startIndex != 0) {
          tasks.add(response.substring(startIndex, endIndex).trim());
        }

        startIndex = endIndex;
        String? nextMatch = match.group(0);
        if (nextMatch != null) {
          startIndex = endIndex + nextMatch.length;
        }
      }

      tasks.add(response.substring(startIndex).trim());
    } catch (e) {
      if (kDebugMode) {
        print('Error getting tasks: $e');
      }
    }

    setState(() {
      _loading = false;
    });

    if (tasks.length < 2) {
      _stopAutonomousMode();
      Future.delayed(const Duration(microseconds: 200), () {
        setState(() {
          _autonomousMode = false;
        });
      });

      textToSpeech
          .speakText('No plan detected! Please input a different message.');
      return [];
    }

    if (_planName == 'Unnamed Plan') {
      textToSpeech.speakText('You plan is ready',
          language: GlobalSettings().selectedLanguage);
    } else {
      textToSpeech.speakText('You plan to $_planName is ready',
          language: GlobalSettings().selectedLanguage);
    }

    return tasks;
  }

  String _lastInputMessage = '';
  String _planName = 'Unnamed Plan';
  List<String> _tasks = [];
  bool _planImplementationInProgress = false;
  bool _stopRequested = false;

  Future<void> _implementPlan() async {
    _stopRequested = false;
    _planImplementationInProgress = true;
    // Send initial message to start working on the plan
    String initialMessage = "Let's work on $_planName. First: ${_tasks[0]}";
    await _sendMessageOnBehalfOfUser(initialMessage,
        customSystemPrompt:
            'You are Glowby, an AI assistant. The user has enabled the auto mode, which allows you to make choices on their behalf. Help the user with the following task and choose only one option, providing a concise action. Your answer should be short and informative. For example, if the task is to book accommodations in Dublin. Please provide a specific hotel name and location that you think the user should book:');

    // Send messages for the rest of the tasks
    for (int i = 1; i < _tasks.length; i++) {
      if (_stopRequested) {
        _stopRequested = false;
        break;
      }
      await textToSpeech.speakText('Moving on to the next task.',
          language: GlobalSettings().selectedLanguage);
      String taskMessage = "Moving on to the next task. ${_tasks[i]}";
      await _sendMessageOnBehalfOfUser(taskMessage,
          customSystemPrompt:
              'You are Glowby, an AI assistant. The user has enabled the auto mode, which allows you to make choices on their behalf. Help the user with the following task and choose only one option, providing a concise action. Your answer should be short and informative. For example, if the task is to book accommodations in Dublin. Please provide a specific hotel name and location that you think the user should book:');
    }

    if (!_stopRequested) {
      // Send the summary message and add a Copy button
      await _sendMessageOnBehalfOfUser('Summarizing...',
          customSystemPrompt:
              'You are Glowby, an AI assistant. The user has enabled the auto mode, and you have followed your suggested concise actions for each task in the plan. Help the user summarize the plan, and provide all info from previous messages but in a shorter but still informative form. ',
          lastMessage: true);
    }
  }

  void insertMessage(String message, String userId, String username) {
    _messages.insert(
        0,
        Message(
          text: message,
          createdAt: Timestamp.now(),
          userId: userId,
          username: username,
        ));
  }

  Future<String> fetchResponseFromAPI(
      String message, String? customSystemPrompt) async {
    List<Map<String, String?>> formattedPreviousMessages = _messages
        .map((message) => {
              'role': message.userId == Ai.defaultUserId ? 'assistant' : 'user',
              'content': message.text
            })
        .toList()
        .reversed
        .toList();

    _currentOperation = OpenAiApi.getResponseFromOpenAI(
      message,
      previousMessages: formattedPreviousMessages,
      customSystemPrompt: customSystemPrompt,
    );

    return await _currentOperation!.value;
  }

  Future<void> _sendMessageOnBehalfOfUser(
    String message, {
    String? customSystemPrompt,
    bool lastMessage = false,
  }) async {
    insertMessage(message, GlobalSettings().userId,
        GlobalSettings().userName); // Insert user's message
    refresh(); // Refresh UI

    String response = await fetchResponseFromAPI(message, customSystemPrompt);

    insertMessage(response, '007',
        widget._name == '' ? 'AI' : widget._name); // Insert AI's response

    // Update the UI
    if (lastMessage) {
      _planImplementationInProgress = false;
    }
    refresh();
    if (!lastMessage) {
      await textToSpeech.speakText(response,
          language: GlobalSettings().selectedLanguage);
    }
  }

  // Inside your _ChatScreenState class

  void setLoading(bool value) {
    setState(() {
      _loading = value;
    });
  }

  void setStopRequested(bool value) {
    setState(() {
      _stopRequested = value;
    });
  }

  void setAutonomousMode(bool value) {
    setState(() {
      _autonomousMode = value;
    });
  }

  void setPlanImplementationInProgress(bool value) {
    setState(() {
      _planImplementationInProgress = value;
    });
  }

  void cancelCurrentOperation() {
    if (_currentOperation != null) {
      _currentOperation!.cancel();
      _currentOperation = null; // Set to null after canceling
    }
  }

  void _stopAutonomousMode() {
    setLoading(false);
    setStopRequested(true);
    setAutonomousMode(false);
    setPlanImplementationInProgress(false);
    cancelCurrentOperation();
  }

  void _showSocialLinksDialog() {
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return AlertDialog(
          title: const Text('Share Glowby'),
          content: SingleChildScrollView(
            child: ListBody(
              children: _buildLinkItems(context),
            ),
          ),
          actions: <Widget>[
            TextButton(
              child: const Text('Close'),
              onPressed: () => Navigator.of(context).pop(),
            ),
          ],
        );
      },
    );
  }

  List<Widget> _buildLinkItems(BuildContext context) {
    final links = [
      {
        'title': 'App Store',
        'url': 'https://apps.apple.com/us/app/glowby-genius/id6446417094'
      },
      {'title': 'Glowby GPT', 'url': 'https://glowbom.com/glowby/gpt'},
      {
        'title': 'Draw-to-code Demo',
        'url': 'https://twitter.com/jacobilin/status/1751365686344155250'
      },
      {'title': 'Website (glowbom.com)', 'url': 'https://glowbom.com/'},
      {'title': 'Twitter: @jacobilin', 'url': 'https://twitter.com/jacobilin'},
      {
        'title': 'Twitter: @GlowbomCorp',
        'url': 'https://twitter.com/GlowbomCorp'
      },
      {
        'title': 'YouTube Channel',
        'url': 'https://www.youtube.com/channel/UCrYQEQPhAHmn7N8W58nNwOw'
      },
      {
        'title': 'GitHub Repository',
        'url': 'https://github.com/glowbom/glowby'
      },
    ];

    return links
        .map((link) => _buildLinkItem(link['title']!, link['url']!, context))
        .toList();
  }

  Widget _buildLinkItem(String text, String url, BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: GestureDetector(
            child: Text(
              text,
              style: const TextStyle(
                color:
                    Colors.black, // Change this color to match your app's theme
                decoration: TextDecoration.underline,
              ),
            ),
            onTap: () async {
              if (await canLaunchUrlString(url)) {
                await launchUrlString(url);
              } else {
                throw 'Could not launch $url';
              }
              if (mounted) {
                Navigator.of(context).pop();
              }
            },
          ),
        ),
        IconButton(
          icon: const Icon(Icons.copy),
          onPressed: () {
            Clipboard.setData(ClipboardData(text: url)).then((value) {
              // Show a snackbar or toast indicating the link was copied
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: Text('Link copied to clipboard!'),
                ),
              );
              Navigator.of(context).pop();
            });
          },
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Container(
        constraints: const BoxConstraints(minWidth: 100, maxWidth: 640),
        child: Column(
          children: <Widget>[
            _loading
                ? const MagicalLoadingView()
                : Expanded(
                    child: Container(
                      child: _autonomousMode
                          ? TasksView(
                              tasks: _tasks,
                              name: _planName,
                              onBackButtonPressed: () {
                                setState(() {
                                  _autonomousMode = false;
                                });
                              },
                              onRequestNewPlanButtonPressed: () async {
                                List<String> tasks =
                                    await _generateTasks(_lastInputMessage);
                                setState(() {
                                  _autonomousMode = true;
                                  _tasks = tasks;
                                });
                              },
                              onImplementPlanButtonPressed: () {
                                setState(() {
                                  _autonomousMode = false;
                                });

                                _implementPlan();
                              },
                            )
                          : Messages(_messages),
                    ),
                  ),
            if (!_autonomousMode && !_loading && !_planImplementationInProgress)
              NewMessage(
                refresh,
                _messages,
                widget._questions,
                widget._name,
                widget._enableAi,
                onAutonomousModeMessage: (String userInput) async {
                  List<String> tasks = await _generateTasks(userInput);
                  setState(() {
                    _autonomousMode = true;
                    _tasks = tasks;
                  });
                },
              ),
            if (!_autonomousMode && !_loading && !_planImplementationInProgress)
              Container(
                margin: const EdgeInsets.all(8),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: <Widget>[
                    // Add the Social Links button
                    /*if (widget._dnsgs! == false)
                      Padding(
                        padding: const EdgeInsets.only(left: 8.0),
                        child: IconButton(
                          icon: const Icon(Icons.share),
                          onPressed: _showSocialLinksDialog,
                        ),
                      ),*/
                    if (widget._allowEnterKey != null && widget._allowEnterKey!)
                      ElevatedButton(
                        onPressed: _showApiKeyDialog,
                        child: const Text(
                          'Enter API Key',
                          style: TextStyle(color: Colors.white),
                        ),
                      ),
                    // Add the AI Settings button conditionally
                    if (OpenAiApi.oat().isNotEmpty)
                      if (widget._showAiSettings != null &&
                          widget._showAiSettings!)
                        Padding(
                          padding: const EdgeInsets.only(left: 8.0),
                          child: ElevatedButton(
                            onPressed: _showAiSettingsDialog,
                            child: const Text(
                              'AI Settings',
                              style: TextStyle(color: Colors.white),
                            ),
                          ),
                        ),
                    /*Padding(
                      padding: const EdgeInsets.only(left: 8.0),
                      child: ElevatedButton(
                        onPressed: () async {
                          String url =
                              'https://apps.apple.com/us/app/glowby-genius/id6446417094';
                          if (await canLaunchUrlString(url)) {
                            await launchUrlString(url);
                          } else {
                            throw 'Could not launch $url';
                          }
                        },
                        child: const Text(
                          'App Store',
                          style: TextStyle(color: Colors.white),
                        ),
                      ),
                    ),
                    Padding(
                      padding: const EdgeInsets.only(left: 8.0),
                      child: ElevatedButton(
                        onPressed: () async {
                          String url = 'https://glowbom.com/glowby/gpt';
                          if (await canLaunchUrlString(url)) {
                            await launchUrlString(url);
                          } else {
                            throw 'Could not launch $url';
                          }
                        },
                        child: const Text(
                          'GPT',
                          style: TextStyle(color: Colors.white),
                        ),
                      ),
                    ),*/
                  ],
                ),
              ),
            if (_planImplementationInProgress)
              const SizedBox(
                height: 50,
                child: Center(
                  child: Text(
                    'Implementing plan...',
                    style: TextStyle(
                      color: Colors.black,
                      fontSize: 20,
                    ),
                  ),
                ),
              ),
            if (_planImplementationInProgress)
              const CircularProgressIndicator(),
            const SizedBox(height: 20),
            // Add the Stop button when plan implementation is in progress
            if (_loading || _planImplementationInProgress)
              Padding(
                padding: const EdgeInsets.only(left: 8.0),
                child: IconButton(
                  icon: const Icon(Icons.stop),
                  onPressed: _stopAutonomousMode,
                  tooltip: 'Stop',
                  color: Colors.black, // Set the color of the stop icon to red
                ),
              ),

            const SizedBox(height: 20),
          ],
        ),
      ),
    );
  }
}
