import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;
import 'package:async/async.dart';
import 'package:glowby/services/hugging_face_api.dart';
import 'package:glowby/services/pulze_ai_api.dart';

class OpenAiApi {
  static final OpenAiApi _instance = OpenAiApi._privateConstructor();
  factory OpenAiApi() => _instance;
  OpenAiApi._privateConstructor();

  String _apiKey = '';
  static int _totalTokensUsed = 0;

  static String oat() => OpenAiApi()._oat();
  static void setOat(String value) => OpenAiApi()._setOat(value);
  static void resetOat() => OpenAiApi()._resetOat();

  void _resetOat() {
    _apiKey = '';
  }

  String _oat() => _apiKey;
  Future<void> _setOat(String value) async {
    _apiKey = value;
    await _secureStorage.write(key: _apiKeyKey, value: _apiKey);
  }

  static String DEFAULT_SYSTEM_PROMPT =
      'You are Glowby, super helpful, nice, and humorous AI assistant ready to help with anything. I like to joke around.';

  static String DEFAULT_SYSTEM_PROMPT_COMPLEX_TASK =
      'You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. For each step, you offer the user 3 options to choose from. Once the user selects an option, you proceed to the next step based on their choice. After the user has chosen an option for the fifth step, you provide them with a customized, actionable plan based on their previous responses. You only reveal the current step and options to ensure an engaging, interactive experience.';
  static String model = 'gpt-3.5-turbo'; //'gpt-4';
  static String selectedLanguage = 'en-US';
  static String systemPrompt = DEFAULT_SYSTEM_PROMPT;
  static const String _apiKeyKey = 'openai_api_key';
  static const String _modelKey = 'openai_model';
  static const String _selectedLanguageKey = 'selected_language';
  static const String _systemPromptKey = 'openai_system_prompt';
  static const FlutterSecureStorage _secureStorage = FlutterSecureStorage();

  static Future<void> loadOat() async {
    try {
      setOat(await _secureStorage.read(key: _apiKeyKey) ?? '');
      model = (await _secureStorage.read(key: _modelKey)) ?? 'gpt-3.5-turbo';
      selectedLanguage =
          (await _secureStorage.read(key: _selectedLanguageKey)) ?? 'en-US';
      systemPrompt = (await _secureStorage.read(key: _systemPromptKey)) ??
          DEFAULT_SYSTEM_PROMPT;
    } catch (e) {
      if (kDebugMode) {
        print('Error loading OAT: $e');
      }
    }

    await HuggingFace_API.loadOat();
    await PulzeAI_API.loadOat();
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
    // Check if the description is safe
    /*bool descriptionIsSafe = await isInputSafe(description);
    if (!descriptionIsSafe) {
      throw Exception(
          'The input provided is not considered safe. Please provide a different input.');
    }*/

    final apiKey = OpenAiApi.oat();

    const queryUrl = 'https://api.openai.com/v1/images/generations';
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

    // Replace this URL with your AWS Lambda function URL
    const lambdaUrl = 'YOUR_LAMBDA_FUNCTION_URL';

    final headers = {
      'Content-Type': 'application/json',
    };

    final data = {
      'input': input,
    };

    try {
      print('calling lambda function with input: $input and url: $lambdaUrl');
      final response = await http.post(
        Uri.parse(lambdaUrl),
        headers: headers,
        body: jsonEncode(data),
      );
      if (kDebugMode) {
        print('isInputSafe response status code: ${response.statusCode}');
        print('isInputSafe response headers: ${response.headers}');
        print('isInputSafe response body: ${response.body}');
      }

      if (response.statusCode == 200) {
        final responseBody = jsonDecode(response.body);
        bool moderationStatus = responseBody['isSafe'];
        return moderationStatus;
      } else {
        if (kDebugMode) {
          print('isInputSafe error: Status code ${response.statusCode}');
        }
        throw Exception('Failed to get response from Lambda function.');
      }
    } catch (e) {
      if (kDebugMode) {
        print('isInputSafe exception: $e');
      }
      rethrow;
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
    String? customSystemPrompt,
  }) {
    // Create a cancelable completer
    final completer = CancelableCompleter<String>();

    if (OpenAiApi.model == 'pulzeai') {
      _getResponseFromPulzeAI(
        message,
        completer,
        previousMessages: previousMessages,
      );
    } else if (OpenAiApi.model == 'huggingface') {
      _getResponseFromHuggingFace(
        message,
        completer,
        previousMessages: previousMessages,
      );
    } else {
      // Wrap the _getResponseFromOpenAI with the cancelable completer
      _getResponseFromOpenAI(
        message,
        completer,
        previousMessages: previousMessages,
        maxTries: maxTries,
        customSystemPrompt: customSystemPrompt,
      );
    }

    return completer.operation;
  }

  static String formatPrevMessages(
      List<Map<String, String?>> previousMessages) {
    return previousMessages.map((message) {
      return "${message['role']}: ${message['content']}";
    }).join(', ');
  }

  static Future<void> _getResponseFromHuggingFace(
    String message,
    CancelableCompleter<String> completer, {
    List<Map<String, String?>> previousMessages = const [],
  }) async {
    String? finalResponse = '';

    if (HuggingFace_API.oat() != '') {
      //print(previousMessages);
      String formattedPrevMessages = formatPrevMessages(previousMessages);
      if (previousMessages.isNotEmpty && HuggingFace_API.sendMessages()) {
        finalResponse = await HuggingFace_API.generate(
            '$message previousMessages: $formattedPrevMessages');
      } else {
        finalResponse = await HuggingFace_API.generate(message);
      }

      //print('finalResponse: $finalResponse');
      if (finalResponse != null) {
        finalResponse = finalResponse
            .replaceAll('assistant: ', '')
            .replaceAll('previousMessages: ', '')
            .replaceAll('user: ', '')
            .replaceAll('[System message]: ', '');
      }
    } else {
      finalResponse =
          'Please enter your Hugging Face Access Token in the settings.';
    }

    completer.complete(finalResponse);

    // Explicitly return null to avoid

    return;
  }

  static Future<void> _getResponseFromPulzeAI(
    String message,
    CancelableCompleter<String> completer, {
    List<Map<String, String?>> previousMessages = const [],
  }) async {
    String? finalResponse = '';

    if (PulzeAI_API.oat() != '') {
      //print(previousMessages);
      String formattedPrevMessages = formatPrevMessages(previousMessages);
      if (previousMessages.isNotEmpty && PulzeAI_API.sendMessages()) {
        finalResponse = await PulzeAI_API.generate(
            '$message previousMessages: $formattedPrevMessages');
      } else {
        finalResponse = await PulzeAI_API.generate(message);
      }

      //print('finalResponse: $finalResponse');
      if (finalResponse != null) {
        finalResponse = finalResponse
            .replaceAll('assistant: ', '')
            .replaceAll('previousMessages: ', '')
            .replaceAll('user: ', '')
            .replaceAll('[System message]: ', '');
      }
    } else {
      finalResponse =
          'Please enter your Puzle AI Access Token in the settings.';
    }

    completer.complete(finalResponse);

    // Explicitly return null to avoid

    return;
  }

  static Future<void> _getResponseFromOpenAI(
      String message, CancelableCompleter<String> completer,
      {List<Map<String, String?>> previousMessages = const [],
      int maxTries = 1,
      String? customSystemPrompt}) async {
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

    final apiKey = OpenAiApi.oat();

    while (tries < maxTries) {
      if (kDebugMode) {
        print('inputMessage = $inputMessage');
      }
      const apiUrl = 'https://api.openai.com/v1/chat/completions';

      final headers = {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $apiKey',
      };

      final adjustedMaxTokens = getAdjustedMaxTokens(inputMessage);

      final data = {
        'model': model,
        'messages': [
          {'role': 'system', 'content': customSystemPrompt ?? systemPrompt},
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
          _totalTokensUsed += tokensUsed;

          // Calculate the cost of the tokens used
          double cost = tokensUsed * 0.002 / 1000;
          if (kDebugMode) {
            // Print the tokens used and the cost to the console
            print('Tokens used in this response: $tokensUsed');
            print('Cost of this response: \$${cost.toStringAsFixed(5)}');
            print('Total tokens used so far: $_totalTokensUsed');
          }

          double totalCost = _totalTokensUsed * 0.002 / 1000;
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
          await Future.delayed(const Duration(seconds: 2));
        } else {
          finalResponse =
              'Sorry, there was an error processing your request. Please try again later.';
          if (kDebugMode) {
            print('Error: $e');
          }
          break;
        }
      }
    }

    completer.complete(finalResponse);

    // Explicitly return null to avoid

    return;
  }

  // Draw to Code Functionality

  Future<String> getHtmlFromOpenAI(
      String imageBase64, String userPrompt) async {
    if (_apiKey == '') {
      return 'Enter API key in settings';
    }

    const systemPrompt = """
You are a skilled web developer with expertise in Tailwind CSS. A user will provide a low-fidelity wireframe along with descriptive notes. Your task is to create a high-fidelity, responsive HTML webpage using Tailwind CSS and JavaScript, embedded within a single HTML file.

- Embed additional CSS and JavaScript directly in the HTML file.
- For images, use placeholders from Unsplash or solid color rectangles.
- Draw inspiration for fonts, colors, and layouts from user-provided style references or wireframes.
- For any previous design iterations, use the provided HTML to refine the design further.
- Apply creative improvements to enhance the design.
- Load JavaScript dependencies through JavaScript modules and unpkg.com.

The final output should be a single HTML file, starting with "<html>". Avoid markdown, excessive newlines, and the character sequence "```".
"""; // The system prompt

    final openAIKey = _apiKey; // Replace with your actual API key
    if (openAIKey.isEmpty) {
      return '';
    }

    final url = Uri.parse("https://api.openai.com/v1/chat/completions");
    var request = http.Request("POST", url)
      ..headers.addAll({
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $openAIKey',
      })
      ..body = jsonEncode({
        "model": "gpt-4-vision-preview",
        "temperature": 0,
        "max_tokens": 4096,
        "messages": [
          {"role": "system", "content": systemPrompt},
          {
            "role": "user",
            "content": [
              {"type": "text", "text": userPrompt},
              {
                "type": "image_url",
                "image_url": {"url": "data:image/jpeg;base64,$imageBase64"}
              }
            ]
          }
        ],
      });

    try {
      final response = await http.Response.fromStream(await request.send());

      if (response.statusCode == 200) {
        final decodedResponse = jsonDecode(response.body);
        // Assuming 'html' is part of the response JSON structure
        String html =
            decodedResponse['choices']?.first['message']['content'] ?? '';
        // Additional logic to handle the HTML content goes here
        return html;
      } else {
        // Handle the error, maybe throw an exception
        print('Failed to get HTML from OpenAI: ${response.body}');
        return '';
      }
    } catch (e) {
      print('Caught error: $e');
      return '';
    }
  }
}
