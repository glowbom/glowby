import 'dart:convert';
import 'package:async/async.dart';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class MultiOnApi {
  static final MultiOnApi _instance = MultiOnApi._privateConstructor();
  factory MultiOnApi() => _instance;
  MultiOnApi._privateConstructor();

  String _apiKey = '';

  static String oat() => MultiOnApi()._oat();
  static void setOat(String value) => MultiOnApi()._setOat(value);
  static void resetOat() => MultiOnApi()._resetOat();

  void _resetOat() {
    _apiKey = '';
  }

  String _oat() => _apiKey;
  Future<void> _setOat(String value) async {
    _apiKey = value;
    await _secureStorage.write(key: _apiKeyKey, value: _apiKey);
  }

  static const String _apiKeyKey = 'multion_api_key';
  static const FlutterSecureStorage _secureStorage = FlutterSecureStorage();

  static Future<void> loadOat() async {
    try {
      setOat(await _secureStorage.read(key: _apiKeyKey) ?? '');
    } catch (e) {
      if (kDebugMode) {
        print('Error loading OAT: $e');
      }
    }
  }

  static CancelableOperation<String> getResponseFromMultiOn(
    String message, {
    List<Map<String, String?>> previousMessages = const [],
    int maxTries = 1,
    String? customSystemPrompt,
  }) {
    // Create a cancelable completer
    final completer = CancelableCompleter<String>();

    // Wrap the _getResponseFromOpenAI with the cancelable completer
    _getResponse(
      message,
      completer,
      previousMessages: previousMessages,
      maxTries: maxTries,
      customSystemPrompt: customSystemPrompt,
    );

    return completer.operation;
  }

  static String _lastSessionId = '';

  static Future<void> _getResponse(
      String message, CancelableCompleter<String> completer,
      {List<Map<String, String?>> previousMessages = const [],
      int maxTries = 1,
      String? customSystemPrompt}) async {
    String finalResponse = '';
    String inputMessage = message;

    // Check if the message is safe
    /*bool messageIsSafe = await isInputSafe(inputMessage);
    if (!messageIsSafe) {
      finalResponse =
          'Sorry, the input provided is not considered safe. Please provide a different input.';
      completer.complete(finalResponse);
      return;
    }*/

    final apiKey = oat();

    if (kDebugMode) {
      print('inputMessage = $inputMessage');
    }
    const apiUrl = 'https://api.multion.ai/v1/web/browse';

    final headers = {
      'Content-Type': 'application/json',
      'X_MULTION_API_KEY': apiKey,
    };

    final data = _lastSessionId.isEmpty
        ? {
            'cmd': inputMessage,
            'include_screenshot': true,
          }
        : {
            'cmd': inputMessage,
            'include_screenshot': true,
            'session_id': _lastSessionId,
          };

    try {
      final response = await http.post(
        Uri.parse(apiUrl),
        headers: headers,
        body: jsonEncode(data),
      );

      if (response.statusCode == 200) {
        final responseBody = jsonDecode(utf8.decode(response.bodyBytes));
        if (kDebugMode) {
          print('responseBody: $responseBody');
        }

        String receivedResponse = responseBody['message'].toString().trim();
        String sessionId = responseBody['session_id'].toString().trim();
        String screenshot = responseBody['screenshot'].toString().trim();

        _lastSessionId = sessionId;

        if (kDebugMode) {
          print('sessionId: $sessionId');
          print('screenshot: $screenshot');
        }

        finalResponse += receivedResponse;

        /*String apiUrlSceenshot =
            'https://api.multion.ai/v1/web/screenshot/$sessionId';

        final headersScreenshot = {
          'X_MULTION_API_KEY': apiKey,
        };

        if (kDebugMode) {
          print('apiUrlSceenshot: $apiUrlSceenshot');
        }

        final responseScreenshot = await http.get(
          Uri.parse(apiUrlSceenshot),
          headers: headersScreenshot,
        );

        if (responseScreenshot.statusCode == 200) {
          final responseBodyScreenshot =
              jsonDecode(utf8.decode(responseScreenshot.bodyBytes));
          if (kDebugMode) {
            print('responseBodyScreenshot: $responseBodyScreenshot');
          }

          String screenshot =
              responseBodyScreenshot['screenshot'].toString().trim();

          finalResponse += '\n\n$screenshot';
        } else {
          throw Exception('Failed to get screenshot from MultiOn API.');
        }*/
      } else {
        throw Exception('Failed to get response from MultiOn API.');
      }
    } catch (e) {
      finalResponse =
          'Sorry, there was an error processing your request. Please try again later.';
      if (kDebugMode) {
        print('Error: $e');
      }
    }

    completer.complete(finalResponse);

    // Explicitly return null to avoid

    return;
  }
}
