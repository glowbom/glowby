import 'dart:async';
import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:glowby/views/screens/global_settings.dart';
import 'package:glowby/views/widgets/paint_window.dart';
import 'package:glowby/utils/utils.dart';

import 'message.dart';
import '../../services/openai_api.dart';
import '../../utils/timestamp.dart';
import 'package:flutter/material.dart';

import '../../models/ai.dart';

/// A class representing the NewMessage widget, which allows users to send messages and receive AI responses.
class NewMessage extends StatefulWidget {
  final Function _refresh;
  final List<Message> _messages;
  final List<Map<String, Object>>? _questions;
  final String? _name;
  final bool? _enableAi;
  final Function(String) onAutonomousModeMessage;

  const NewMessage(this._refresh, this._messages, this._questions, this._name,
      this._enableAi,
      {super.key, required this.onAutonomousModeMessage});

  @override
  _NewMessageState createState() => _NewMessageState();
}

class _NewMessageState extends State<NewMessage> {
  late Ai ai;

  final _controller = TextEditingController();
  var _enteredMessage = '';

  FocusNode? _focusNode;
  bool _isRecording = false;
  Timer? _voiceCancelTimer;
  bool _isProcessing = false;
  bool _stopRequested = false;

  void _onVoiceReady(text) {
    if (_isRecording) {
      if (_voiceCancelTimer != null) {
        _voiceCancelTimer!.cancel();
        _voiceCancelTimer = null;
      }

      _controller.value = TextEditingValue(text: text);
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
      Utils.initializeState(_onVoiceReady);
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

    Utils.recordVoice(GlobalSettings().selectedLanguage);

    setState(() {
      _isRecording = true;
    });

    if (_voiceCancelTimer != null) {
      _voiceCancelTimer!.cancel();
      _voiceCancelTimer = null;
    }

    _voiceCancelTimer = Timer(const Duration(seconds: 8), () {
      if (_isRecording) {
        setState(() {
          _isRecording = false;
          _voiceCancelTimer = null;
        });
      }
    });
  }

  void _sendMessage() async {
    _stopRequested = false;
    FocusScope.of(context).unfocus();

    _addUserMessageToChat();

    final message = _enteredMessage.trim();
    _resetMessageInput();

    if (Utils.isImageGenerationCommand(message)) {
      await handleImageGenerationCommand(message);
    }
    // Check if Autonomous mode is on
    else if (GlobalSettings().autonomousMode) {
      handleAutonomousMode(
          message); // Call the callback function with the user's input
    } else {
      await _processUserMessage(message);
    }
  }

  void _resetMessageInput() {
    _controller.value = TextEditingValue.empty;
    _focusNode!.requestFocus();

    _enteredMessage = '';
  }

  void _addUserMessageToChat() {
    widget._messages.insert(
      0,
      Message(
          text: _enteredMessage.trim(),
          createdAt: Timestamp.now(),
          userId: GlobalSettings().userId,
          username: GlobalSettings().userName),
    );
  }

  Future<void> _processUserMessage(String message) async {
    // Add a new message instance indicating that the AI is typing
    Message typingMessage = Message(
      text: 'typing...',
      createdAt: Timestamp.now(),
      userId: Ai.defaultUserId,
      username: widget._name == '' ? 'AI' : widget._name,
    );
    widget._messages.insert(0, typingMessage);
    widget._refresh();

    // Select the last 5 messages (excluding the user's input message)
    int messageHistoryCount = min(20, widget._messages.length - 1);
    List<Message> previousMessages =
        widget._messages.sublist(1, messageHistoryCount + 1);

    // Convert previousMessages to the format expected by the API
    List<Map<String, String?>> formattedPreviousMessages = previousMessages
        .map((message) {
          return {
            'role': message.userId == Ai.defaultUserId ? 'assistant' : 'user',
            'content': message.text
          };
        })
        .toList()
        .reversed
        .toList();

    setState(() {
      _isProcessing = true; // Set to true before processing
    });

    var response = await ai.message(message,
        previousMessages: formattedPreviousMessages,
        aiEnabled: widget._enableAi == null ? true : widget._enableAi!);

    if (_stopRequested) {
      return;
    }

    setState(() {
      _isProcessing = false; // Set to false after processing
    });

    // Remove the typing message instance when the response is received
    widget._messages.remove(typingMessage);

    if (response.isNotEmpty) {
      for (Message m in response) {
        widget._messages.insert(
          0,
          m,
        );
      }
    }

    widget._refresh();
  }

  Future<void> handleAutonomousMode(String message) async {
    widget.onAutonomousModeMessage(message);
  }

  Future<void> handleImageGenerationCommand(String message) async {
    final pattern = Utils.getMatchingPattern(message);
    final description = pattern != null
        ? message.replaceAll(RegExp(pattern, caseSensitive: false), '').trim()
        : '';
    //print('description: $description');
    //print('enableAi: ${widget._enableAi}');
    if (description.isNotEmpty &&
        (widget._enableAi == null || widget._enableAi!)) {
      Message drawingMessage = Message(
        text: Utils.getRandomImageGenerationFunnyMessage(),
        createdAt: Timestamp.now(),
        userId: Ai.defaultUserId,
        username: widget._name == '' ? 'AI' : widget._name,
      );
      widget._messages.insert(0, drawingMessage);
      widget._refresh();

      // Generate the image
      try {
        final imageUrl = (await OpenAiApi.generateImageUrl(description))!;
        Message message = Message(
          text: 'Here is your image!',
          createdAt: Timestamp.now(),
          userId: Ai.defaultUserId,
          username: widget._name == '' ? 'AI' : widget._name,
          link: imageUrl,
        );

        widget._messages.remove(drawingMessage);
        widget._messages.insert(0, message);
        widget._messages.insert(
            0,
            Message(
              text: Utils.getRandomImageReadyMessage(),
              createdAt: Timestamp.now(),
              userId: Ai.defaultUserId,
              username: widget._name == '' ? 'AI' : widget._name,
            ));

        widget._refresh();

        Utils.downloadImage(imageUrl, description);
      } catch (e) {
        // Handle the exception and emit an error state
        widget._messages.remove(drawingMessage);
        Message message = Message(
          text: 'Something went wrong. Please try again later.',
          createdAt: Timestamp.now(),
          userId: Ai.defaultUserId,
          username: widget._name == '' ? 'AI' : widget._name,
        );

        widget._messages.remove(drawingMessage);
        widget._messages.insert(0, message);
        widget._refresh();
      }
    }
  }

  void _stopProcessing() {
    // Set the stop requested flag
    _stopRequested = true;

    // Use setState to update the state and UI accordingly
    setState(() {
      // Set the processing flag to false
      _isProcessing = false;

      // If there's a typing message, remove it
      if (widget._messages.isNotEmpty &&
          widget._messages[0].text == "typing...") {
        widget._messages.removeAt(0);
      }

      // Refresh the widget to reflect the changes
      widget._refresh();
    });

    // Cancel any ongoing network operation if it exists
    ai.getCurrentNetworkOperation()?.cancel();
  }

  /* method for opening a paint window */
  void _openPaintWindow() {
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return const PaintWindow();
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(top: 8),
      padding: const EdgeInsets.all(8),
      child: Row(
        children: <Widget>[
          Expanded(
            child: Padding(
              padding: const EdgeInsets.only(bottom: 20),
              child: TextField(
                cursorColor: Theme.of(context).primaryColor,
                textInputAction: TextInputAction.send,
                focusNode: _focusNode,
                autofocus: true,
                controller: _controller,
                textCapitalization: TextCapitalization.sentences,
                autocorrect: true,
                enableSuggestions: true,
                style: TextStyle(color: Theme.of(context).primaryColor),
                decoration: const InputDecoration(labelText: 'Send message...'),
                onChanged: (value) {
                  setState(() {
                    _enteredMessage = value;
                  });
                },
                onSubmitted: (value) {
                  if (_enteredMessage.trim().isNotEmpty) {
                    _sendMessage();
                    // Clear the content of the TextEditingController after sending the message
                    Future.delayed(const Duration(microseconds: 500), () {
                      _controller.clear();
                    });
                  }
                },
                keyboardType: TextInputType.multiline,
                maxLines: 9,
                minLines: 1,
              ),
            ),
          ),
          if (kIsWeb && !_isProcessing)
            IconButton(
              color: Theme.of(context).primaryColor,
              icon: Icon(
                _isRecording ? Icons.record_voice_over : Icons.keyboard_voice,
              ),
              onPressed: _voiceMessage,
            ),
          if (!_isProcessing)
            IconButton(
              color: Theme.of(context).primaryColor,
              icon: const Icon(
                Icons.brush,
              ),
              onPressed: _openPaintWindow,
            ),
          if (_isProcessing)
            IconButton(
              color: Theme.of(context).primaryColor,
              icon: const Icon(Icons.stop),
              onPressed: _stopProcessing,
            ),
          IconButton(
            color: Theme.of(context).primaryColor,
            icon: const Icon(
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
