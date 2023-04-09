import 'dart:convert';
import 'dart:math';

import 'package:flutter/material.dart';

import 'chat_screen.dart';

void main() => runApp(Talk(null));

class _TalkState extends State<Talk> {
  var _appScreen = 'Loading';

  String? _title;
  String? _mainColor;
  bool? _voice = false;

  Future<dynamic> loadContentFromAssets() async {
    String data =
        await DefaultAssetBundle.of(context).loadString("assets/talk.glowbom");
    return json.decode(data);
  }

  @override
  void initState() {
    super.initState();

    if (_content != null) {
      _questions = _content['questions'];
      if (_content.containsKey('title')) {
        _title = _content['title'];
      }

      if (_content.containsKey('main_color')) {
        _mainColor = _content['main_color'];
      } else {
        _mainColor = 'Blue';
      }

      if (_content.containsKey('voice')) {
        _voice = _content['voice'];
      } else {
        _voice = false;
      }

      _pressed100();
    } else {
      loadContentFromAssets().then((value) => setState(() {
            _content = value;
            if (_content.containsKey('title')) {
              _title = _content['title'];
              print('title: ' + _title!);
            }

            if (_content.containsKey('main_color')) {
              _mainColor = _content['main_color'];
            } else {
              _mainColor = 'Blue';
            }

            if (_content.containsKey('voice')) {
              _voice = _content['voice'];
            } else {
              _voice = false;
            }

            _questions = List<Map<String, Object>>.empty(growable: true);
            List<dynamic> list = _content['questions'];
            for (int i = 0; i < list.length; i++) {
              dynamic item = list[i];
              Map<String, Object> question = {
                "title": item['title'].toString(),
                "description": item['description'].toString(),
                "buttonsTexts": List<String>.from(item['buttonsTexts']),
                "buttonAnswers": List<int>.from(item['buttonAnswers']),
                "answersCount": item['answersCount'],
                "goIndexes": List<int>.from(item['goIndexes']),
                "answerPicture": item['answerPicture'].toString(),
                "answerPictureDelay": item['answerPictureDelay'],
                "goConditions": [],
                "heroValues": [],
                "picturesSpriteNames": ["", "", "", "", "", ""]
              };
              _questions.add(question.cast<String, Object>());
            }
            _pressed100();
          }));
    }
  }

  var _content;

  _TalkState(this._content);

  var _questions;

  List<Map<String, Object>> deepCopy(List<Map<String, Object>> items) {
    List<Map<String, Object>> result = List.from(
      {},
    );

    // Go through all elements.
    for (var i = 0; i < items.length; i++) {
      result.add(Map.from(items[i]));
    }

    return result;
  }

  void _pressed100() {
    bool? dnsgs = _content != null && _content.containsKey('dnsgs')
        ? _content['dnsgs']
        : false;

    if (dnsgs == true) {
      setState(() {
        _appScreen = 'Test100';
      });
    } else {
      setState(() {
        _appScreen = 'Glowbom';
      });
      Future.delayed(const Duration(milliseconds: 1500), () {
        setState(() {
          _appScreen = 'Test100';
        });
      });
    }
  }

  int tintValue(int value, double factor) =>
      max(0, min((value + ((255 - value) * factor)).round(), 255));

  Color tintColor(Color color, double factor) => Color.fromRGBO(
      tintValue(color.red, factor),
      tintValue(color.green, factor),
      tintValue(color.blue, factor),
      1);

  int shadeValue(int value, double factor) =>
      max(0, min(value - (value * factor).round(), 255));

  Color shadeColor(Color color, double factor) => Color.fromRGBO(
      shadeValue(color.red, factor),
      shadeValue(color.green, factor),
      shadeValue(color.blue, factor),
      1);

  MaterialColor generateMaterialColor(Color color) {
    return MaterialColor(color.value, {
      50: tintColor(color, 0.9),
      100: tintColor(color, 0.8),
      200: tintColor(color, 0.6),
      300: tintColor(color, 0.4),
      400: tintColor(color, 0.2),
      500: color,
      600: shadeColor(color, 0.1),
      700: shadeColor(color, 0.2),
      800: shadeColor(color, 0.3),
      900: shadeColor(color, 0.4),
    });
  }

  @override
  Widget build(BuildContext context) {
    //speak();
    return MaterialApp(
      title: 'Chat',
      theme: ThemeData(
        primarySwatch: generateMaterialColor(_mainColor == 'Green'
            ? Color.fromRGBO(85, 185, 158, 1)
            : _mainColor == 'Blue'
                ? Colors.blue
                : _mainColor == 'Red'
                    ? Colors.red
                    : _mainColor == 'Black'
                        ? Colors.black
                        : Colors.grey),
      ),
      home: Scaffold(
          appBar: AppBar(
            title: Text(
              _title != null ? _title! : 'Chat App',
              style: TextStyle(
                color: Colors.white,
              ),
            ),
            centerTitle: true,
          ),
          body: _appScreen == 'Loading'
              ? Center(
                  child: Text('Loading...'),
                )
              : _appScreen == 'Glowbom'
                  ? Center(
                      child:
                          const Image(image: AssetImage('assets/glowbom.png')),
                    )
                  : ChatScreen(
                      _content != null && _content.containsKey('start_over')
                          ? _content['start_over']
                          : 'AI',
                      _questions,
                      _voice!,
                    )),
    );
  }
}

class Talk extends StatefulWidget {
  final _content;

  Talk(this._content);

  @override
  State<StatefulWidget> createState() {
    return _TalkState(_content);
  }
}
