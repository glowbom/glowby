import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';

import 'message.dart';
import 'new_message.dart';
import 'messages.dart';
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
  final TextToSpeech textToSpeech = TextToSpeech(); // Initialize TextToSpeech

  List<Message> _messages = [];

  // Refresh the chat screen and handle text-to-speech functionality
  void refresh() {
    if (widget._voice) {
      try {
        if (_messages.isNotEmpty && _messages[0].userId == '007') {
          textToSpeech.speakText(_messages[0].text);
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
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
