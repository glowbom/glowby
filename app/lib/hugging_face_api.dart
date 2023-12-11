import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class HuggingFace_API {
  static final HuggingFace_API _instance =
      HuggingFace_API._privateConstructor();
  factory HuggingFace_API() => _instance;
  HuggingFace_API._privateConstructor();

  String _apiKey = '';

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

  static String oat() => HuggingFace_API()._oat();
  static void setOat(String value) => HuggingFace_API()._setOat(value);
  static void resetOat() => HuggingFace_API()._resetOat();

  void _resetOat() {
    _apiKey = '';
  }

  String _oat() => _apiKey;
  Future<void> _setOat(String value) async {
    _apiKey = value;
    await _secureStorage.write(key: 'huggingface_api_key', value: _apiKey);
  }

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

  static Future<void> setSendMessages(bool sendMessages) async {
    _sendMessages = sendMessages;
    await _secureStorage
        .write(key: _sendMessagesKey, value: _sendMessages.toString())
        .then((value) => null);
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
    if (!_isValidTokenAndModel(modelId)) {
      return 'Invalid token or model ID';
    }

    final response = await _makeRequest(modelId, text);
    return _processResponse(response, template);
  }

  static bool _isValidTokenAndModel(String modelId) {
    return oat() != '' && modelId != '';
  }

  static Future<http.Response> _makeRequest(String modelId, String text) async {
    final queryUrl = 'https://api-inference.huggingface.co/models/$modelId';
    final headers = {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer ${oat()}',
    };
    final body = jsonEncode({
      'inputs': _systemMessage.isEmpty
          ? text
          : '$text [System message]: $_systemMessage',
    });

    return await http.post(Uri.parse(queryUrl), headers: headers, body: body);
  }

  static String? _processResponse(http.Response response, String template) {
    if (response.statusCode != 200) {
      return 'Error processing request';
    }
    final jsonResponse = jsonDecode(response.body);
    final templateJson = jsonDecode(template);
    return _findValueByTemplate(jsonResponse, templateJson);
  }
}
