import 'package:flutter/material.dart';

import 'ai_settings_dialog.dart';
import 'message.dart';
import 'new_message.dart';
import 'messages.dart';
import 'openai_api.dart';
import 'text_to_speech.dart'; // Import the new TextToSpeech class
import 'api_key_dialog.dart'; // Import the ApiKeyDialog widget

class ChatScreen extends StatefulWidget {
  final List<Map<String, Object>> _questions;
  final String _name;
  final bool _voice;

  ChatScreen(this._name, this._questions, this._voice);

  @override
  _ChatScreenState createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
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

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Container(
        constraints: BoxConstraints(minWidth: 100, maxWidth: 640),
        child: Column(
          children: <Widget>[
            Expanded(
              child: Container(
                child: Messages(_messages),
              ),
            ),
            NewMessage(
              refresh,
              _messages,
              widget._questions,
              widget._name,
            ),
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
