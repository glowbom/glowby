import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class HuggingFaceApi {
  static final HuggingFaceApi _instance = HuggingFaceApi._privateConstructor();
  factory HuggingFaceApi() => _instance;
  HuggingFaceApi._privateConstructor();

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
  static const FlutterSecureStorage _secureStorage = FlutterSecureStorage();

  static String oat() => HuggingFaceApi()._oat();
  static void setOat(String value) => HuggingFaceApi()._setOat(value);
  static void resetOat() => HuggingFaceApi()._resetOat();

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
    if (value is List && template is List) {
      return _processListByTemplate(value, template);
    } else if (value is Map && template is Map) {
      return _processMapByTemplate(
          value as Map<String, dynamic>, template as Map<String, dynamic>);
    } else if (template == "***") {
      return value.toString();
    }

    return null;
  }

  static String? _processListByTemplate(
      List<dynamic> valueList, List<dynamic> templateList) {
    if (valueList.length != templateList.length) return null;

    for (int i = 0; i < valueList.length; i++) {
      final result = _findValueByTemplate(valueList[i], templateList[i]);
      if (result != null) {
        return result;
      }
    }

    return null;
  }

  static String? _processMapByTemplate(
      Map<String, dynamic> valueMap, Map<String, dynamic> templateMap) {
    for (var key in templateMap.keys) {
      final result = _findValueByTemplate(valueMap[key], templateMap[key]);
      if (result != null) {
        return result;
      }
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

    try {
      return await http.post(Uri.parse(queryUrl), headers: headers, body: body);
    } on http.ClientException catch (e) {
      // Handle the exception related to the HTTP client
      if (kDebugMode) {
        print('ClientException occurred: $e');
      }
      // Consider re-throwing the exception or returning an error response
    } catch (e) {
      // Handle other types of exceptions
      if (kDebugMode) {
        print('An error occurred: $e');
      }
      // Consider re-throwing the exception or returning an error response
    }

    // If an error occurs, you might want to return a default response:
    return http.Response('{"error": "An unexpected error occurred."}', 500);
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
