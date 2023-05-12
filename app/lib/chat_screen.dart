import 'package:flutter/material.dart';
import 'package:web/tasks_view.dart';

import 'ai.dart';
import 'ai_settings_dialog.dart';
import 'magical_loading_view.dart';
import 'message.dart';
import 'new_message.dart';
import 'messages.dart';
import 'openai_api.dart';
import 'text_to_speech.dart'; // Import the new TextToSpeech class
import 'api_key_dialog.dart';
import 'timestamp.dart'; // Import the ApiKeyDialog widget
import 'package:async/async.dart';

class ChatScreen extends StatefulWidget {
  final List<Map<String, Object>> _questions;
  final String _name;
  final bool _voice;
  final String? _selectedModel;
  final String? _selectedLanguage;
  final String? _systemPrompt;
  final bool? _allowEnterKey;
  final bool? _allowDataImport;
  final bool? _autonomousMode;
  final bool? _enableAi;
  final bool? _showAiSettings;

  ChatScreen(
      this._name,
      this._questions,
      this._voice,
      this._selectedModel,
      this._selectedLanguage,
      this._systemPrompt,
      this._allowEnterKey,
      this._allowDataImport,
      this._autonomousMode,
      this._enableAi,
      this._showAiSettings);

  @override
  _ChatScreenState createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  var _autonomousMode = false;
  bool _loading = false;
  bool _voiceEnabled = true;
  void updateVoiceEnabled(bool value) {
    setState(() {
      _voiceEnabled = value;
    });
  }

  final TextToSpeech textToSpeech = TextToSpeech(); // Initialize TextToSpeech

  List<Message> _messages = [];

  @override
  void initState() {
    super.initState();
    _voiceEnabled = widget._voice;
    AiSettingsDialog.loadDialogValues(widget._selectedModel,
        widget._selectedLanguage, widget._systemPrompt, widget._autonomousMode);

    OpenAI_API.loadOat().then((_) {
      setState(() {});
    });
  }

  // Refresh the chat screen and handle text-to-speech functionality
  void refresh() {
    if (widget._voice && _voiceEnabled) {
      try {
        if (_messages.isNotEmpty &&
            _messages[0].userId == '007' &&
            _planImplementationInProgress == false) {
          textToSpeech.speakText(_messages[0].text,
              language: AiSettingsDialog.selectedLanguage);
        }
      } catch (e) {
        print('Error: $e'); // Log the exception
      }
    }
    setState(() {});
  }

  void _showApiKeyDialog() {
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return ApiKeyDialog(); // Use the ApiKeyDialog widget
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
      _currentOperation = await OpenAI_API.getResponseFromOpenAI(
          _lastInputMessage,
          customSystemPrompt:
              'You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. The steps should be concise.');

      String response = await _currentOperation!.value;

      if (response ==
          'Sorry, there was an error processing your request. Please try again later.') {
        _stopAutonomousMode();
        Future.delayed(Duration(microseconds: 200), () {
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
      print('Error getting tasks: $e');
    }

    setState(() {
      _loading = false;
    });

    if (tasks.length < 2) {
      _stopAutonomousMode();
      Future.delayed(Duration(microseconds: 200), () {
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
          language: AiSettingsDialog.selectedLanguage);
    } else {
      textToSpeech.speakText('You plan to ${_planName} is ready',
          language: AiSettingsDialog.selectedLanguage);
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
      textToSpeech.speakText('Moving on to the next task.',
          language: AiSettingsDialog.selectedLanguage);
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

  Future<void> _sendMessageOnBehalfOfUser(
    String message, {
    String? customSystemPrompt,
    bool lastMessage = false,
  }) async {
    // Add the message to the list
    _messages.insert(
        0,
        Message(
          text: message,
          createdAt: Timestamp.now(),
          userId: 'Me',
          username: 'Me',
        ));

    //await textToSpeech.speakText(message,
    //  language: AiSettingsDialog.selectedLanguage);

    refresh();

    String response = '';

    // Convert previousMessages to the format expected by the API
    List<Map<String, String?>> formattedPreviousMessages = _messages
        .map((message) {
          return {
            'role': message.userId == Ai.defaultUserId ? 'assistant' : 'user',
            'content': message.text
          };
        })
        .toList()
        .reversed
        .toList();

    _currentOperation = await OpenAI_API.getResponseFromOpenAI(message,
        previousMessages: formattedPreviousMessages,
        customSystemPrompt: customSystemPrompt);

    response = await _currentOperation!.value;

    // Add the response to the list
    _messages.insert(
        0,
        Message(
          text: response,
          createdAt: Timestamp.now(),
          userId: '007',
          username: widget._name == '' ? 'AI' : widget._name,
        ));

    // Update the UI
    if (lastMessage) {
      _planImplementationInProgress = false;
    }
    refresh();
    if (!lastMessage) {
      await textToSpeech.speakText(response,
          language: AiSettingsDialog.selectedLanguage);
    }
  }

  void _stopAutonomousMode() {
    setState(() {
      _loading = false;
      _stopRequested = true;
      _autonomousMode = false;
      _planImplementationInProgress = false;
      _currentOperation?.cancel();
    });
  }

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Container(
        constraints: BoxConstraints(minWidth: 100, maxWidth: 640),
        child: Column(
          children: <Widget>[
            _loading
                ? MagicalLoadingView()
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
                margin: EdgeInsets.all(8),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: <Widget>[
                    ElevatedButton(
                      child: Text('Enter API Key'),
                      onPressed: _showApiKeyDialog,
                    ),
                    // Add the AI Settings button conditionally
                    if (OpenAI_API.oat().isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(left: 8.0),
                        child: ElevatedButton(
                          child: Text('AI Settings'),
                          onPressed: _showAiSettingsDialog,
                        ),
                      ),
                  ],
                ),
              ),
            if (_planImplementationInProgress)
              Container(
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
            if (_planImplementationInProgress) CircularProgressIndicator(),
            SizedBox(height: 20),
            // Add the Stop button when plan implementation is in progress
            if (_loading || _planImplementationInProgress)
              Padding(
                padding: const EdgeInsets.only(left: 8.0),
                child: IconButton(
                  icon: Icon(Icons.stop),
                  onPressed: _stopAutonomousMode,
                  tooltip: 'Stop',
                  color: Colors.black, // Set the color of the stop icon to red
                ),
              ),

            SizedBox(height: 20),
          ],
        ),
      ),
    );
  }
}
