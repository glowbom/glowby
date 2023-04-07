import 'package:flutter/cupertino.dart';
import 'package:flutter_tts/flutter_tts.dart';

import 'message.dart';
import 'new_message.dart';
import 'messages.dart';
import 'package:flutter/material.dart';

class ChatScreen extends StatefulWidget {
  final List<Map<String, Object>> _questions;
  final String _name;
  final bool _voice;

  ChatScreen(this._name, this._questions, this._voice);

  @override
  _ChatScreenState createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  FlutterTts flutterTts;

  Future s(text) async {
    if (flutterTts == null) {
      flutterTts = FlutterTts();
    }

    //List<dynamic> languages = await flutterTts.getLanguages;

    //print(languages);

    if ((text as String).startsWith('Italian: ')) {
      await flutterTts.setLanguage("it-IT");
      text = (text as String).replaceAll('Italian: ', '');
    } else if ((text as String).startsWith('German: ')) {
      await flutterTts.setLanguage("de-DE");
      text = (text as String).replaceAll('German: ', '');
    } else if ((text as String).startsWith('Portuguese: ')) {
      await flutterTts.setLanguage("pt-PT");
      text = (text as String).replaceAll('Portuguese: ', '');
    } else if ((text as String).startsWith('Dutch: ')) {
      await flutterTts.setLanguage("nl-NL");
      text = (text as String).replaceAll('Dutch: ', '');
    } else if ((text as String).startsWith('Russian: ')) {
      await flutterTts.setLanguage("ru-RU");
      text = (text as String).replaceAll('Russian: ', '');
    } else if ((text as String).startsWith('American Spanish: ')) {
      await flutterTts.setLanguage("es-US");
      text = (text as String).replaceAll('American Spanish: ', '');
    } else if ((text as String).startsWith('Mexican Spanish: ')) {
      await flutterTts.setLanguage("es-MX");
      text = (text as String).replaceAll('Mexican Spanish: ', '');
    } else if ((text as String).startsWith('Canadian French: ')) {
      await flutterTts.setLanguage("fr-CA");
      text = (text as String).replaceAll('Canadian French: ', '');
    } else if ((text as String).startsWith('French: ')) {
      await flutterTts.setLanguage("fr-FR");
      text = (text as String).replaceAll('French: ', '');
    } else if ((text as String).startsWith('Spanish: ')) {
      await flutterTts.setLanguage("es-ES");
      text = (text as String).replaceAll('Spanish: ', '');
    } else if ((text as String).startsWith('American English: ')) {
      await flutterTts.setLanguage("en-US");
      text = (text as String).replaceAll('American English: ', '');
    } else if ((text as String).startsWith('British English: ')) {
      await flutterTts.setLanguage("en-GB");
      text = (text as String).replaceAll('British English: ', '');
    } else if ((text as String).startsWith('Australian English: ')) {
      await flutterTts.setLanguage("en-AU");
      text = (text as String).replaceAll('Australian English: ', '');
    } else if ((text as String).startsWith('English: ')) {
      await flutterTts.setLanguage("en-US");
      text = (text as String).replaceAll('English: ', '');
    }

    await flutterTts.awaitSpeakCompletion(true);
    await flutterTts.speak(text);
  }

  List<Message> _messages = [];

  void refresh() {
    if (widget._voice && _messages.length > 0) {
      try {
        if (_messages[0].userId == '007') {
          s(_messages[0].text);
        }
      } catch (e) {}
    }
    setState(() {});
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
          ],
        ),
      ),
    );
  }
}
