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

class ChatScreen extends StatefulWidget {
  final List<Map<String, Object>> _questions;
  final String _name;
  final bool _voice;

  ChatScreen(this._name, this._questions, this._voice);

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
    OpenAI_API.loadOat().then((_) {
      setState(() {});
    });
  }

  // Refresh the chat screen and handle text-to-speech functionality
  void refresh() {
    if (widget._voice && _voiceEnabled) {
      try {
        if (_messages.isNotEmpty && _messages[0].userId == '007') {
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
      String response = await OpenAI_API.getResponseFromOpenAI(
          _lastInputMessage,
          customSystemPrompt:
              'You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan.');

      print('response: $response');
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

    return tasks;
  }

  String _lastInputMessage = '';
  String _planName = 'Unnamed Plan';
  List<String> _tasks = [];

  Future<void> _implementPlan() async {
    // Send initial message to start working on the plan
    String initialMessage = "Let's work on $_planName. First: ${_tasks[0]}";
    await _sendMessageOnBehalfOfUser(initialMessage);

    // Send messages for the rest of the tasks
    for (int i = 1; i < _tasks.length; i++) {
      String taskMessage = "Now let's do ${_tasks[i]}";
      await _sendMessageOnBehalfOfUser(taskMessage);
    }

    // Generate the summary message
    String summary = "Here's the summary of the plan:\n\n";
    for (int i = 0; i < _tasks.length; i++) {
      summary += "${i + 1}. ${_tasks[i]}\n";
    }

    // Send the summary message and add a Copy button
    await _sendMessageOnBehalfOfUser(summary);
  }

  Future<void> _sendMessageOnBehalfOfUser(String message) async {
    // Add the message to the list
    _messages.insert(
        0,
        Message(
          text: message,
          createdAt: Timestamp.now(),
          userId: 'Me',
          username: 'Me',
        ));

    // Get the AI response
    String response = await OpenAI_API.getResponseFromOpenAI(message);

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
    refresh();
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
            if (!_autonomousMode && !_loading)
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
            if (!_autonomousMode && !_loading)
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
            SizedBox(height: 20),
          ],
        ),
      ),
    );
  }
}
