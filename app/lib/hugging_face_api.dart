import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class HuggingFace_API {
  static String apiKey = '';
  static const String _apiKeyKey = 'huggingface_api_key';
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
    apiKey = await _secureStorage.read(key: _apiKeyKey) ?? '';
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

  // Examples:
  // generate('facebook/bart-large-cnn', 'What\'s the best way to play a guitar?', '[{"summary_text": "***"}]');
  // generate('google/flan-t5-large', 'What\'s the best way to play a guitar?', '[{"generated_text": "***"}]');
  static Future<String?> generate(
      String modelId, String text, String template) async {
    final queryUrl = 'https://api-inference.huggingface.co/models/$modelId';
    final headers = {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $apiKey',
    };

    final body = jsonEncode({
      'inputs': text,
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
      throw Exception('Failed to generate summary');
    }
  }
}
