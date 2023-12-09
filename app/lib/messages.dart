import 'package:glowby/global_settings.dart';

import 'message.dart';
import 'message_bubble.dart';
import 'package:flutter/material.dart';

/// A class representing the Messages widget, which displays a list of MessageBubble widgets.
class Messages extends StatefulWidget {
  final List<Message> _messages;
  final ScrollController controller = ScrollController();

  Messages(this._messages);

  @override
  _MessagesState createState() => _MessagesState();
}

class _MessagesState extends State<Messages> {
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
      if (messageText.startsWith(prefix))
        messageText = messageText.replaceAll(prefix, '');
    }

    return messageText;
  }

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      reverse: true,
      itemCount: widget._messages.length,
      controller: widget.controller,
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
