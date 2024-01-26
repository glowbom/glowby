import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:async/async.dart';
import 'package:glowby/services/pulze_ai_api.dart';

class Network {
  static final Network _instance = Network._privateConstructor();
  factory Network() => _instance;
  Network._privateConstructor();

  static String defaultSystemPrompt =
      'You are Glowby, super helpful, nice, and humorous AI assistant ready to help with anything. I like to joke around.';

  static String defaultSystemPromptComplexTask =
      'You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. For each step, you offer the user 3 options to choose from. Once the user selects an option, you proceed to the next step based on their choice. After the user has chosen an option for the fifth step, you provide them with a customized, actionable plan based on their previous responses. You only reveal the current step and options to ensure an engaging, interactive experience.';
  static String model = 'gpt-3.5-turbo'; //'gpt-4';
  static String selectedLanguage = 'en-US';
  static String systemPrompt = defaultSystemPrompt;
  static const String _modelKey = 'openai_model';
  static const String _selectedLanguageKey = 'selected_language';
  static const String _systemPromptKey = 'openai_system_prompt';
  static const FlutterSecureStorage _secureStorage = FlutterSecureStorage();

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

  static CancelableOperation<String> getResponseFromPulze(
    String message, {
    List<Map<String, String?>> previousMessages = const [],
    int maxTries = 1,
    String? customSystemPrompt,
  }) {
    // Create a cancelable completer
    final completer = CancelableCompleter<String>();

    _getResponseFromPulzeAI(
      message,
      completer,
      previousMessages: previousMessages,
    );

    return completer.operation;
  }

  static String formatPrevMessages(
      List<Map<String, String?>> previousMessages) {
    return previousMessages.map((message) {
      return "${message['role']}: ${message['content']}";
    }).join(', ');
  }

  static Future<void> _getResponseFromPulzeAI(
    String message,
    CancelableCompleter<String> completer, {
    List<Map<String, String?>> previousMessages = const [],
  }) async {
    String? finalResponse = '';

    if (PulzeAiApi.oat() != '') {
      //print(previousMessages);
      String formattedPrevMessages = formatPrevMessages(previousMessages);
      if (previousMessages.isNotEmpty && PulzeAiApi.sendMessages()) {
        finalResponse = await PulzeAiApi.generate(
            '$message previousMessages: $formattedPrevMessages');
      } else {
        finalResponse = await PulzeAiApi.generate(message);
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
}
