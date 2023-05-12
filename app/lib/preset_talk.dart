import 'dart:convert';
import 'package:flutter/material.dart';
import 'color_utils.dart';
import 'chat_screen.dart';
import 'dart:html' as html;

var _content;

class _TalkState extends State<Talk> {
  var _appScreen = 'Loading';

  String? _title;
  String? _mainColor;
  bool? _voice = false;
  List<Map<String, Object>> _questions = [];
  String? _selectedModel;
  String? _selectedLanguage;
  String? _systemPrompt;
  bool? _allowEnterKey;
  bool? _allowDataImport;
  bool? _autonomousMode;
  bool? _enableAi;
  bool? _showAiSettings;

  _TalkState();

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
      _questions = buildQuestions(_content['questions']);
      _title = _content['title'];
      _mainColor = _content['main_color'] ?? 'Blue';
      _voice = _content['voice'] ?? false;
      _selectedModel = _content['ai_selected_model'];
      _selectedLanguage = _content['ai_selected_language'];
      _systemPrompt = _content['ai_system_prompt'];
      _allowEnterKey = _content['ai_allow_enter_key'] ?? true;
      _allowDataImport = _content['ai_allow_data_import'] ?? true;
      _autonomousMode = _content['ai_autonomous_mode'] ?? false;
      _enableAi = _content['ai_enable'] ?? true;
      _showAiSettings = _content['ai_show_settings'] ?? true;

      _pressed100();
    } else {
      loadContentFromAssets().then((value) => setState(() {
            _content = value;
            _title = _content['title'];
            _mainColor = _content['main_color'] ?? 'Blue';
            _voice = _content['voice'] ?? false;
            _selectedModel = _content['ai_selected_model'];
            _selectedLanguage = _content['ai_selected_language'];
            _systemPrompt = _content['ai_system_prompt'];
            _allowEnterKey = _content['ai_allow_enter_key'] ?? true;
            _allowDataImport = _content['ai_allow_data_import'] ?? true;
            _autonomousMode = _content['ai_autonomous_mode'] ?? false;
            _enableAi = _content['ai_enable'] ?? true;
            _showAiSettings = _content['ai_show_settings'] ?? true;

            _questions = buildQuestions(_content['questions']);
            _pressed100();
          }));
    }
  }

  void _startFilePicker() async {
    try {
      html.FileUploadInputElement uploadInput = html.FileUploadInputElement();
      uploadInput.multiple = false;
      uploadInput.accept = '.glowbom';

      uploadInput.onChange.listen((e) {
        final files = uploadInput.files;
        if (files != null && files.isNotEmpty) {
          final file = files.first;
          final reader = html.FileReader();

          reader.onLoadEnd.listen((e) {
            String content = reader.result as String;
            _content = json.decode(content);
            keyIndex.value += 1;
          });

          reader.readAsText(file);
        }
      });

      uploadInput.click();
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
      debugShowCheckedModeBanner: false,
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

ValueNotifier<int> keyIndex = ValueNotifier<int>(0);

// A StatefulWidget that represents the main Talk widget.
class Talk extends StatefulWidget {
  final Key key;

  Talk({required this.key});

  @override
  State<StatefulWidget> createState() {
    return _TalkState();
  }
}

class TalkApp extends StatefulWidget {
  @override
  _TalkAppState createState() => _TalkAppState();
}

class _TalkAppState extends State<TalkApp> {
  @override
  void initState() {
    super.initState();
    keyIndex.addListener(() {
      setState(() {});
    });
  }

  @override
  Widget build(BuildContext context) {
    return Talk(
      key: Key(keyIndex.value.toString()),
    );
  }
}
