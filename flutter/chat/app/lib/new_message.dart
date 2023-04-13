//@JS()
//library tv;
// Uncomment 2 lines above to compile the web version

import 'dart:async';

import 'package:flutter/foundation.dart';

import 'message.dart';
import 'timestamp.dart';
import 'package:flutter/material.dart';

// Uncomment the next line to compile the web version
//import 'package:js/js.dart';

import 'ai.dart';

// Uncomment the next block to compile the web version

/*@JS()
external int rv();

/// Allows assigning a function to be callable from `window.functionName()`
@JS('vr')
external set _vr(void Function(dynamic) f);

/// Allows calling the assigned function from Dart as well.
@JS()
external void vr(text);
*/

/// A class representing the NewMessage widget, which allows users to send messages and receive AI responses.
class NewMessage extends StatefulWidget {
  final Function _refresh;
  final List<Message> _messages;
  final List<Map<String, Object>>? _questions;
  final String? _name;

  NewMessage(
    this._refresh,
    this._messages,
    this._questions,
    this._name,
  );

  @override
  _NewMessageState createState() => _NewMessageState();
}

class _NewMessageState extends State<NewMessage> {
  late var ai;

  final _controller = new TextEditingController();
  var _enteredMessage = '';

  FocusNode? _focusNode;
  bool _isRecording = false;
  Timer? _voiceCancelTimer;

  void _onVoiceReady(text) {
    if (_isRecording) {
      if (_voiceCancelTimer != null) {
        _voiceCancelTimer!.cancel();
        _voiceCancelTimer = null;
      }

      _controller.value = text;
      _enteredMessage = text;
      _sendMessage();

      setState(() {
        _isRecording = false;
      });
    }
  }

  @override
  void initState() {
    if (kIsWeb) {
      // Uncomment the next line to compile the web version
      //_vr = allowInterop(_onVoiceReady);
    }
    ai = Ai(
      widget._name,
      widget._questions,
    );
    super.initState();

    _focusNode = FocusNode();
  }

  @override
  void dispose() {
    // Clean up the focus node when the Form is disposed.
    _focusNode!.dispose();

    super.dispose();
  }

  void _voiceMessage() {
    if (_isRecording) {
      return;
    }

    // Uncomment for web version
    //rv();

    setState(() {
      _isRecording = true;
    });

    if (_voiceCancelTimer != null) {
      _voiceCancelTimer!.cancel();
      _voiceCancelTimer = null;
    }

    _voiceCancelTimer = Timer(Duration(seconds: 8), () {
      if (_isRecording) {
        setState(() {
          _isRecording = false;
          _voiceCancelTimer = null;
        });
      }
    });
  }

  void _sendMessage() async {
    FocusScope.of(context).unfocus();

    widget._messages.insert(
      0,
      Message(
          text: _enteredMessage.trim(),
          createdAt: Timestamp.now(),
          userId: 'Me',
          username: 'Me'),
    );

    final message = _enteredMessage.trim();
    _controller.value = TextEditingValue.empty;
    _focusNode!.requestFocus();

    _enteredMessage = '';

    // Add a new message instance indicating that the AI is typing
    Message typingMessage = Message(
      text: 'typing...',
      createdAt: Timestamp.now(),
      userId: Ai.defaultUserId,
      username: widget._name == '' ? 'AI' : widget._name,
    );
    widget._messages.insert(0, typingMessage);
    widget._refresh();

    var response = await ai.message(message);

    // Remove the typing message instance when the response is received
    widget._messages.remove(typingMessage);

    if (response.length > 0) {
      for (Message m in response) {
        widget._messages.insert(
          0,
          m,
        );
      }
    }

    widget._refresh();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: EdgeInsets.only(top: 8),
      padding: EdgeInsets.all(8),
      child: Row(
        children: <Widget>[
          Expanded(
            child: Padding(
              padding: const EdgeInsets.only(bottom: 20),
              child: TextField(
                textInputAction: TextInputAction.none,
                focusNode: _focusNode,
                autofocus: true,
                controller: _controller,
                textCapitalization: TextCapitalization.sentences,
                autocorrect: true,
                enableSuggestions: true,
                decoration: InputDecoration(labelText: 'Send message...'),
                onChanged: (value) {
                  setState(() {
                    _enteredMessage = value;
                  });
                },
                onSubmitted: (value) {
                  if (_enteredMessage.trim().isNotEmpty) {
                    _sendMessage();
                  }
                },
              ),
            ),
          ),
          if (kIsWeb)
            IconButton(
              color: Theme.of(context).primaryColor,
              icon: Icon(
                _isRecording ? Icons.record_voice_over : Icons.keyboard_voice,
              ),
              onPressed: _voiceMessage,
            ),
          IconButton(
            color: Theme.of(context).primaryColor,
            icon: Icon(
              Icons.send,
            ),
            onPressed: _enteredMessage.trim().isEmpty ? null : _sendMessage,
          ),
        ],
      ),
    );
  }
}

class MediaType {}
