import 'dart:convert';
import 'dart:typed_data';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'color_utils.dart';
import 'chat_screen.dart';

class _TalkState extends State<Talk> {
  var _appScreen = 'Loading';

  String? _title;
  String? _mainColor;
  bool? _voice = false;
  List<Map<String, Object>> _questions = [];
  GlobalKey<ChatScreenState> chatScreenKey = GlobalKey();

  _TalkState(this._content);

  var _content;

  Future<dynamic> loadContentFromAssets() async {
    String data =
        await DefaultAssetBundle.of(context).loadString("assets/talk.glowbom");
    return json.decode(data);
  }

  @override
  void initState() {
    super.initState();
    initializeTalkState();
  }

  void initializeTalkState() {
    if (_content != null) {
      _questions = _content['questions'];
      _title = _content['title'];
      _mainColor = _content['main_color'] ?? 'Blue';
      _voice = _content['voice'] ?? false;
      _pressed100();
    } else {
      loadContentFromAssets().then((value) => setState(() {
            _content = value;
            _title = _content['title'];
            _mainColor = _content['main_color'] ?? 'Blue';
            _voice = _content['voice'] ?? false;
            _questions = buildQuestions(_content['questions']);
            _pressed100();
          }));
    }
  }

  void _startFilePicker() async {
    try {
      FilePickerResult? result = await FilePicker.platform.pickFiles(
        type: FileType.custom,
        allowedExtensions: ['glowbom'], // Limit the file extension to 'glowbom'
      );

      if (result != null) {
        PlatformFile file = result.files.first;
        Uint8List? fileBytes = file.bytes;
        if (fileBytes != null) {
          String content =
              utf8.decode(fileBytes); // Decode the Uint8List to a String
          Map<String, dynamic> fileContent = json.decode(content);

          // Do something with the file content, e.g., update the questions
          setState(() {
            _content = fileContent;
            _title = _content['title'];
            _mainColor = _content['main_color'] ?? 'Blue';
            _voice = _content['voice'] ?? false;
            _questions = buildQuestions(_content['questions']);
          });

          print(_content);
          print('name = ${_content['start_over']}');
        }
      }
    } catch (e) {
      print('Error: $e'); // Log the exception
    }
  }

  List<Map<String, Object>> buildQuestions(List<dynamic> questionsData) {
    List<Map<String, Object>> questions =
        List<Map<String, Object>>.empty(growable: true);
    for (int i = 0; i < questionsData.length; i++) {
      dynamic item = questionsData[i];
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
      questions.add(question);
    }
    return questions;
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

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Chat',
      theme: ThemeData(
        textSelectionTheme: TextSelectionThemeData(
          selectionColor: Colors.grey, // Change this to your desired color
        ),
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
          actions: [
            IconButton(
              icon: Icon(Icons.file_upload),
              onPressed: _startFilePicker,
            ),
          ],
        ),
        body: _appScreen == 'Loading'
            ? Center(
                child: Text('Loading...'),
              )
            : _appScreen == 'Glowbom'
                ? Center(
                    child: const Image(image: AssetImage('assets/glowbom.png')),
                  )
                : ChatScreen(
                    _content != null && _content.containsKey('start_over')
                        ? _content['start_over']
                        : 'AI',
                    _questions,
                    _voice!,
                  ),
      ),
    );
  }
}

// A StatefulWidget that represents the main Talk widget.
class Talk extends StatefulWidget {
  final _content;

  Talk(this._content);

  @override
  State<StatefulWidget> createState() {
    return _TalkState(_content);
  }
}
