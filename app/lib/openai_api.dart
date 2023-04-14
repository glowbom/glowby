import 'dart:convert';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;

int totalTokensUsed = 0;

class OpenAI_API {
  static String apiKey = '';
  static String model = 'gpt-3.5-turbo'; //'gpt-4';
  static String systemPrompt =
      'You are Glowby, super helpful, nice, and humorous AI assistant ready to help with anything. I like to joke around.';
  static const String _apiKeyKey = 'openai_api_key';
  static const String _modelKey = 'openai_model';
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
    systemPrompt = (await _secureStorage.read(key: _systemPromptKey)) ??
        'You are Glowby, super helpful, nice, and humorous AI assistant ready to help with anything. I like to joke around.';
  }

  static Future<void> setModel(String value) async {
    model = value;
    await _secureStorage.write(key: _modelKey, value: model);
  }

  static Future<void> setSystemPrompt(String value) async {
    systemPrompt = value;
    await _secureStorage.write(key: _systemPromptKey, value: systemPrompt);
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
      'size': '1024x1024',
    });

    print('Request URL: $queryUrl');
    print('Request Headers: $headers');
    print('Request Body: $body');

    final response =
        await http.post(Uri.parse(queryUrl), headers: headers, body: body);

    print('Response Status Code: ${response.statusCode}');
    print('Response Body: ${response.body}');

    if (response.statusCode == 200) {
      final jsonResponse = jsonDecode(response.body);
      final imageUrl = jsonResponse['data'][0]['url'];
      print('Generated Image URL: $imageUrl');

      return imageUrl;
    } else {
      throw Exception('Failed to generate image');
    }
  }

  static Future<bool> isInputSafe(String input) async {
    print('isInputSafe called with input: $input');
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

      print('isInputSafe response: ${response.body}');

      if (response.statusCode == 200) {
        final responseBody = jsonDecode(response.body);
        String? moderationStatus = responseBody['data']['classification'];
        return moderationStatus == 'safe';
      } else {
        print('isInputSafe error: Status code ${response.statusCode}');
        throw Exception('Failed to get response from OpenAI Moderation API.');
      }
    } catch (e) {
      print('isInputSafe exception: $e');
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

  static Future<String> getResponseFromOpenAI(String message,
      {List<Map<String, String?>> previousMessages = const [],
      int maxTries = 1}) async {
    String finalResponse = '';
    String inputMessage = message;
    int tries = 0;

    while (tries < maxTries) {
      print('inputMessage = $inputMessage');
      final apiUrl = 'https://api.openai.com/v1/chat/completions';

      final headers = {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $apiKey',
      };

      final adjustedMaxTokens = getAdjustedMaxTokens(inputMessage);

      final data = {
        'model': model,
        'messages': [
          {'role': 'system', 'content': systemPrompt},
          ...previousMessages,
          {'role': 'user', 'content': inputMessage}
        ],
        'max_tokens': adjustedMaxTokens,
        'n': 1,
        'stop': null,
        'temperature': 1,
      };

      final response = await http.post(
        Uri.parse(apiUrl),
        headers: headers,
        body: jsonEncode(data),
      );

      if (response.statusCode == 200) {
        final responseBody = jsonDecode(utf8.decode(response.bodyBytes));
        String receivedResponse =
            responseBody['choices'][0]['message']['content'].toString().trim();

        // Add the current received response to the final response
        finalResponse += receivedResponse;

        // Add the tokens used in this response to the total tokens used
        int tokensUsed = responseBody['usage']['total_tokens'];
        totalTokensUsed += tokensUsed;

        // Calculate the cost of the tokens used
        double cost = tokensUsed * 0.002 / 1000;

        // Print the tokens used and the cost to the console
        print('Tokens used in this response: $tokensUsed');
        print('Cost of this response: \$${cost.toStringAsFixed(5)}');
        print('Total tokens used so far: $totalTokensUsed');

        double totalCost = totalTokensUsed * 0.002 / 1000;
        print('Total cost so far: \$${totalCost.toStringAsFixed(5)}');

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
    }

    return finalResponse;
  }
}