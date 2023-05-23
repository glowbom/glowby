import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class HuggingFace_API {
  static String apiKey = '';
  static String _template = '''[
  {
    "generated_text": "***"
  }
]
''';
  static String _model = 'google/flan-t5-large';
  static String _systemMessage = '';
  static bool _sendMessages = false;
  static const String _apiKeyKey = 'huggingface_api_key';
  static const String _templateKey = 'huggingface_template';
  static const String _modelKey = 'huggingface_model';
  static const String _systemMessageKey = 'huggingface_system_message';
  static const String _sendMessagesKey = 'huggingface_send_messages';
  static final FlutterSecureStorage _secureStorage = FlutterSecureStorage();

  static String oat() {
    if (apiKey == '') {
      loadOat();
    }

    return apiKey;
  }

  static void setOat(String value) async {
    apiKey = value;
    await _secureStorage.write(key: _apiKeyKey, value: apiKey);
  }

  static Future<void> loadOat() async {
    try {
      apiKey = await _secureStorage.read(key: _apiKeyKey) ?? '';
      _template = await _secureStorage.read(key: _templateKey) ??
          '''[
  {
    "generated_text": "***"
  }
]
''';
      _model =
          await _secureStorage.read(key: _modelKey) ?? 'google/flan-t5-large';
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

  static void setSendMessages(bool sendMessages) {
    _sendMessages = sendMessages;
    _secureStorage.write(
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

  static String? _findValueByTemplate(dynamic value, dynamic template) {
    if (value is List && template is List && value.length == template.length) {
      for (int i = 0; i < value.length; i++) {
        final result = _findValueByTemplate(value[i], template[i]);
        if (result != null) {
          return result;
        }
      }
    } else if (value is Map && template is Map) {
      for (var key in template.keys) {
        final result = _findValueByTemplate(value[key], template[key]);
        if (result != null) {
          return result;
        }
      }
    } else if (template == "***") {
      return value;
    }

    return null;
  }

  static Future<String?> generate(String text) async {
    return await _generate(_model, text, _template);
  }

  // Examples:
  // generate('facebook/bart-large-cnn', 'What\'s the best way to play a guitar?', '[{"summary_text": "***"}]');
  // generate('google/flan-t5-large', 'What\'s the best way to play a guitar?', '[{"generated_text": "***"}]');
  static Future<String?> _generate(
      String modelId, String text, String template) async {
    if (apiKey == '') {
      return 'Please enter your Hugging Face Access Token in the settings.';
    }

    if (modelId == '') {
      return 'Please enter Model ID in the settings.';
    }

    final queryUrl = 'https://api-inference.huggingface.co/models/$modelId';
    final headers = {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $apiKey',
    };

    final body = jsonEncode({
      'inputs': _systemMessage == ''
          ? text
          : text + ' [System message]: ' + _systemMessage,
    });

    if (kDebugMode) {
      print('Request URL: $queryUrl');
    }

    final response =
        await http.post(Uri.parse(queryUrl), headers: headers, body: body);

    if (kDebugMode) {
      print('Response Status Code: ${response.statusCode}');
      print('Response Body: ${response.body}');
    }

    if (response.statusCode == 200) {
      final jsonResponse = jsonDecode(response.body);
      final templateJson = jsonDecode(template);
      final generatedText = _findValueByTemplate(jsonResponse, templateJson);

      if (kDebugMode) {
        print('Generated Text: $generatedText');
      }

      return generatedText;
    } else {
      return 'Sorry, there was an error processing your request. Please try again later.';
    }
  }
}
