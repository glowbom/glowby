import 'package:glowby/views/screens/global_settings.dart';

import 'message.dart';
import 'message_bubble.dart';
import 'package:flutter/material.dart';

/// A class representing the Messages widget, which displays a list of MessageBubble widgets.
class Messages extends StatefulWidget {
  final List<Message> _messages;

  const Messages(this._messages, {super.key});

  @override
  MessagesState createState() => MessagesState();
}

class MessagesState extends State<Messages> {
  late ScrollController _controller;

  @override
  void initState() {
    super.initState();
    _controller = ScrollController();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// Replaces language prefixes in the message text.
  String _processMessageText(String messageText) {
    const List<String> languagePrefixes = [
      'Italian: ',
      'German: ',
      'Portuguese: ',
      'Dutch: ',
      'Russian: ',
      'American Spanish: ',
      'Mexican Spanish: ',
      'Canadian French: ',
      'French: ',
      'Spanish: ',
      'American English: ',
      'Australian English: ',
      'British English: ',
      'English: ',
    ];

    for (final prefix in languagePrefixes) {
      if (messageText.startsWith(prefix)) {
        // Replace only the first occurrence of the prefix
        messageText = messageText.replaceFirst(prefix, '');
        break; // Since we found the prefix, no need to check the rest
      }
    }

    return messageText;
  }

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      reverse: true,
      itemCount: widget._messages.length,
      controller: _controller,
      itemBuilder: (ctx, index) {
        final message = widget._messages[index];
        final processedText = _processMessageText(message.text);

        return MessageBubble(
          processedText,
          message.username,
          message.userId == GlobalSettings().userId,
          message.link,
          key: ValueKey(message.createdAt.toString()),
        );
      },
    );
  }
}
