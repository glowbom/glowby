import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;
import 'package:async/async.dart';

int totalTokensUsed = 0;

class OpenAI_API {
  static String DEFAULT_SYSTEM_PROMPT =
      'You are Glowby, super helpful, nice, and humorous AI assistant ready to help with anything. I like to joke around.';

  static String DEFAULT_SYSTEM_PROMPT_COMPLEX_TASK =
      'You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. For each step, you offer the user 3 options to choose from. Once the user selects an option, you proceed to the next step based on their choice. After the user has chosen an option for the fifth step, you provide them with a customized, actionable plan based on their previous responses. You only reveal the current step and options to ensure an engaging, interactive experience.';
  static String apiKey = '';
  static String model = 'gpt-3.5-turbo'; //'gpt-4';
  static String selectedLanguage = 'en-US';
  static String systemPrompt = DEFAULT_SYSTEM_PROMPT;
  static const String _apiKeyKey = 'openai_api_key';
  static const String _modelKey = 'openai_model';
  static const String _selectedLanguageKey = 'selected_language';
  static const String _systemPromptKey = 'openai_system_prompt';
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
    model = (await _secureStorage.read(key: _modelKey)) ?? 'gpt-3.5-turbo';
    selectedLanguage =
        (await _secureStorage.read(key: _selectedLanguageKey)) ?? 'en-US';
    systemPrompt = (await _secureStorage.read(key: _systemPromptKey)) ??
        DEFAULT_SYSTEM_PROMPT;
  }

  static Future<void> setModel(String value) async {
    model = value;
    await _secureStorage.write(key: _modelKey, value: model);
  }

  static Future<void> setSystemPrompt(String value) async {
    systemPrompt = value;
    await _secureStorage.write(key: _systemPromptKey, value: systemPrompt);
  }

  static Future<void> setSelectedLanguage(String value) async {
    selectedLanguage = value;
    await _secureStorage.write(
        key: _selectedLanguageKey, value: selectedLanguage);
  }

  static Future<String?> generateImageUrl(String description) async {
    final queryUrl = 'https://api.openai.com/v1/images/generations';
    final headers = {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $apiKey',
    };

    final body = jsonEncode({
      'prompt': description,
      'n': 1,
      'size': '512x512',
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
      final imageUrl = jsonResponse['data'][0]['url'];
      if (kDebugMode) {
        print('Generated Image URL: $imageUrl');
      }

      return imageUrl;
    } else {
      throw Exception('Failed to generate image');
    }
  }

  static Future<bool> isInputSafe(String input) async {
    if (kDebugMode) {
      print('isInputSafe called with input: $input');
    }
    final apiUrl = 'https://api.openai.com/v1/moderation/classify';

    final headers = {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $apiKey',
    };

    final data = {
      'model': 'content-moderator',
      'prompt': input,
    };

    try {
      final response = await http.post(
        Uri.parse(apiUrl),
        headers: headers,
        body: jsonEncode(data),
      );
      if (kDebugMode) {
        print('isInputSafe response: ${response.body}');
      }

      if (response.statusCode == 200) {
        final responseBody = jsonDecode(response.body);
        String? moderationStatus = responseBody['data']['classification'];
        return moderationStatus == 'safe';
      } else {
        if (kDebugMode) {
          print('isInputSafe error: Status code ${response.statusCode}');
        }
        throw Exception('Failed to get response from OpenAI Moderation API.');
      }
    } catch (e) {
      if (kDebugMode) {
        print('isInputSafe exception: $e');
      }
      throw e;
    }
  }

  static int getAdjustedMaxTokens(String inputText,
      {int defaultMaxTokens = 300}) {
    List<String> keywords = [
      'code',
      'snippet',
      'class',
      'function',
      'method',
      'generate',
      'create',
      'build',
      'implement',
      'algorithm',
      'example',
      'template',
      'sample',
      'skeleton',
      'structure',
    ];

    bool containsKeyword(String text, List<String> keywords) {
      return keywords.any((keyword) => text.toLowerCase().contains(keyword));
    }

    // Increase max tokens if the input text contains any of the keywords
    if (containsKeyword(inputText, keywords)) {
      return defaultMaxTokens *
          3; // Example: increase max tokens by a factor of 3
    }

    return defaultMaxTokens;
  }

  static CancelableOperation<String> getResponseFromOpenAI(
    String message, {
    List<Map<String, String?>> previousMessages = const [],
    int maxTries = 1,
    String? customSystemPrompt = null,
  }) {
    // Create a cancelable completer
    final completer = CancelableCompleter<String>();

    // Wrap the _getResponseFromOpenAI with the cancelable completer
    _getResponseFromOpenAI(
      message,
      completer,
      previousMessages: previousMessages,
      maxTries: maxTries,
      customSystemPrompt: customSystemPrompt,
    );

    return completer.operation;
  }

  static Future<void> _getResponseFromOpenAI(
      String message, CancelableCompleter<String> completer,
      {List<Map<String, String?>> previousMessages = const [],
      int maxTries = 1,
      String? customSystemPrompt = null}) async {
    String finalResponse = '';
    String inputMessage = message;
    int tries = 0;

    // Check if the message is safe
    /*bool messageIsSafe = await isInputSafe(inputMessage);
    if (!messageIsSafe) {
      finalResponse =
          'Sorry, the input provided is not considered safe. Please provide a different input.';
      completer.complete(finalResponse);
      return;
    }*/

    while (tries < maxTries) {
      if (kDebugMode) {
        print('inputMessage = $inputMessage');
      }
      final apiUrl = 'https://api.openai.com/v1/chat/completions';

      final headers = {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $apiKey',
      };

      final adjustedMaxTokens = getAdjustedMaxTokens(inputMessage);

      final data = {
        'model': model,
        'messages': [
          {
            'role': 'system',
            'content':
                customSystemPrompt == null ? systemPrompt : customSystemPrompt
          },
          ...previousMessages,
          {'role': 'user', 'content': inputMessage}
        ],
        'max_tokens': adjustedMaxTokens,
        'n': 1,
        'stop': null,
        'temperature': 1,
      };

      try {
        final response = await http.post(
          Uri.parse(apiUrl),
          headers: headers,
          body: jsonEncode(data),
        );

        if (response.statusCode == 200) {
          final responseBody = jsonDecode(utf8.decode(response.bodyBytes));
          String receivedResponse = responseBody['choices'][0]['message']
                  ['content']
              .toString()
              .trim();

          // Add the current received response to the final response
          finalResponse += receivedResponse;

          // Add the tokens used in this response to the total tokens used
          int tokensUsed = responseBody['usage']['total_tokens'];
          totalTokensUsed += tokensUsed;

          // Calculate the cost of the tokens used
          double cost = tokensUsed * 0.002 / 1000;
          if (kDebugMode) {
            // Print the tokens used and the cost to the console
            print('Tokens used in this response: $tokensUsed');
            print('Cost of this response: \$${cost.toStringAsFixed(5)}');
            print('Total tokens used so far: $totalTokensUsed');
          }

          double totalCost = totalTokensUsed * 0.002 / 1000;
          if (kDebugMode) {
            print('Total cost so far: \$${totalCost.toStringAsFixed(5)}');
          }

          // Check if the received response was cut-off
          if (responseBody['choices'][0]['finish_reason'] == 'length') {
            // Use the last part of the received response as input for the next request
            inputMessage += receivedResponse;
            int maxLength = 1024 * 10; // You can set this to a desired limit
            if (inputMessage.length > maxLength) {
              inputMessage =
                  inputMessage.substring(inputMessage.length - maxLength);
            }
            tries++;
          } else {
            break;
          }
        } else {
          throw Exception('Failed to get response from OpenAI API.');
        }
      } catch (e) {
        if (tries + 1 < maxTries) {
          tries++;
          // You can add a delay before retrying the request.
          await Future.delayed(Duration(seconds: 2));
        } else {
          finalResponse =
              'Sorry, there was an error processing your request. Please try again later.';
          if (kDebugMode) {
            print('Error: $e');
          }
        }
      }
    }

    completer.complete(finalResponse);

    // Explicitly return null to avoid

    return null;
  }
}
