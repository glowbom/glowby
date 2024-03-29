import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class PulzeAiApi {
  static final PulzeAiApi _instance = PulzeAiApi._privateConstructor();
  factory PulzeAiApi() => _instance;
  PulzeAiApi._privateConstructor();

  String _apiKey = '';

  static String oat() => PulzeAiApi()._oat();
  static void setOat(String value) => PulzeAiApi()._setOat(value);
  static void resetOat() => PulzeAiApi()._resetOat();

  void _resetOat() {
    _apiKey = '';
  }

  String _oat() => _apiKey;
  Future<void> _setOat(String value) async {
    _apiKey = value;
    await _secureStorage.write(key: _apiKeyKey, value: _apiKey);
  }

  static String _template = '''[
  {
    "generated_text": "***"
  }
]
''';
  static String _model = 'pulze-v0';
  static String _lastUsedModel = 'pulze-v0';
  static String _systemMessage = '';
  static bool _sendMessages = false;
  static const String _apiKeyKey = 'pulze_ai_api_key';
  static const String _templateKey = 'pulze_ai_template';
  static const String _modelKey = 'pulze_ai_model';
  static const String _systemMessageKey = 'pulze_ai_system_message';
  static const String _sendMessagesKey = 'pulze_ai_send_messages';
  static const FlutterSecureStorage _secureStorage = FlutterSecureStorage();

  static Future<void> loadOat() async {
    try {
      setOat(await _secureStorage.read(key: _apiKeyKey) ?? '');
      _template = await _secureStorage.read(key: _templateKey) ??
          '''[
  {
    "generated_text": "***"
  }
]
''';
      _model = await _secureStorage.read(key: _modelKey) ?? 'pulze-v0';
      _systemMessage = await _secureStorage.read(key: _systemMessageKey) ?? '';
      _sendMessages =
          await _secureStorage.read(key: _sendMessagesKey) == 'true';
    } catch (e) {
      if (kDebugMode) {
        print('Error loading OAT: $e');
      }
    }
  }

  static bool sendMessages() {
    return _sendMessages;
  }

  static Future<void> setSendMessages(bool sendMessages) async {
    _sendMessages = sendMessages;
    await _secureStorage.write(
        key: _sendMessagesKey, value: _sendMessages.toString());
  }

  static String systemMessage() {
    return _systemMessage;
  }

  static void setSystemMessage(systemMessage) {
    _systemMessage = systemMessage;
    _secureStorage.write(key: _systemMessageKey, value: _systemMessage);
  }

  static String model() {
    return _model;
  }

  static String lastUsedModel() {
    return _lastUsedModel;
  }

  static void setModel(model) {
    _model = model;
    _secureStorage.write(key: _modelKey, value: _model);
  }

  static String template() {
    return _template;
  }

  static void setTemplate(template) {
    _template = template;
    _secureStorage.write(key: _templateKey, value: _template);
  }

  static Future<String?> generate(String text) async {
    return await _generate(_model, text, _template);
  }

  static Future<String?> _generate(
      String modelId, String text, String template) async {
    if (PulzeAiApi.oat() == '') {
      return 'Please enter your Pulze AI Access Token in the settings.';
    }

    final apiKey = PulzeAiApi.oat();

    const queryUrl = 'https://api.pulze.ai/v1/completions/';
    final headers = {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $apiKey',
      'Pulze-Labels': '{"hello": "world"}', // Added Pulze-Labels header
    };

    final body = jsonEncode({
      'model': modelId, // Specify the model
      'prompt': _systemMessage == ''
          ? text
          : '$text [System message]: $_systemMessage',
      'max_tokens': 300, // Added max_tokens
      'temperature': 0, // Added temperature
    });

    try {
      if (kDebugMode) {
        print('Request URL: $queryUrl');
      }

      var response =
          await http.post(Uri.parse(queryUrl), headers: headers, body: body);

      if (kDebugMode) {
        print('Response Status Code: ${response.statusCode}');
        print('Response Body: ${response.body}');
      }

      // If there's a redirect, follow it
      if (response.statusCode == 307) {
        var newUri = response.headers['location'];
        if (kDebugMode) {
          print('New Uri: $newUri');
        }
        if (newUri != null) {
          response = await http.get(Uri.parse(newUri));
        }

        if (kDebugMode) {
          print('Response Status Code: ${response.statusCode}');
          print('Response Body: ${response.body}');
        }
      }

      if (response.statusCode == 200) {
        final responseBody = jsonDecode(utf8.decode(response.bodyBytes));
        // Extracting the model information
        String modelInfo = responseBody['metadata']['model']['model'];
        _lastUsedModel = modelInfo;

        String receivedResponse =
            responseBody['choices'][0]['text'].toString().trim();

        // Add the current received response to the final response
        final generatedText = receivedResponse;

        if (kDebugMode) {
          print('Generated Text: $generatedText');
          print('Model Info: $modelInfo');
        }

        return generatedText;
      } else {
        return 'Sorry, there was an error processing your request. Please try again later.';
      }
    } catch (e) {
      if (kDebugMode) {
        print('An exception occurred: $e');
      }
      return 'An error occurred while processing your request.';
    }
  }
}
