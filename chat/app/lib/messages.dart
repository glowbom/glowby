import 'message.dart';
import 'message_bubble.dart';
import 'package:flutter/material.dart';

class Messages extends StatefulWidget {
  final List<Message> _messages;
  final ScrollController controller = new ScrollController();
  Messages(this._messages);

  @override
  _MessagesState createState() => _MessagesState();
}

class _MessagesState extends State<Messages> {
  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      reverse: true,
      itemCount: widget._messages.length,
      controller: widget.controller,
      itemBuilder: (ctx, index) => MessageBubble(
        widget._messages[index].text
            .replaceAll('Italian: ', '')
            .replaceAll('German: ', '')
            .replaceAll('Portuguese: ', '')
            .replaceAll('Dutch: ', '')
            .replaceAll('Russian: ', '')
            .replaceAll('American Spanish: ', '')
            .replaceAll('Mexican Spanish: ', '')
            .replaceAll('Canadian French: ', '')
            .replaceAll('French: ', '')
            .replaceAll('Spanish: ', '')
            .replaceAll('American English: ', '')
            .replaceAll('Australian English: ', '')
            .replaceAll('British English: ', '')
            .replaceAll('English: ', ''),
        widget._messages[index].username,
        widget._messages[index].userId == 'Me',
        widget._messages[index].link,
        key: ValueKey(widget._messages[index].createdAt.toString()),
      ),
    );
  }
}
