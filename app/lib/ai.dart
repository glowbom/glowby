import 'dart:math';

import 'package:glowby/hugging_face_api.dart';

import 'openai_api.dart';
import 'timestamp.dart';
import 'message.dart';
import 'package:async/async.dart';

/// A class representing the AI chatbot that processes and responds to user messages.
class Ai {
  final List<Map<String, Object>>? _questions;
  final String? _name;
  CancelableOperation<String>? newtworkOperation;

  CancelableOperation<String>? getCurrentNetworkOperation() {
    return newtworkOperation;
  }

  static const String defaultUserId = '007';

  Ai(this._name, this._questions);

  /// Processes the user's message and returns an AI-generated response.
  ///
  /// [message] is the input message from the user.
  Future<List<Message>> message(
    String message, {
    List<Map<String, String?>> previousMessages = const [],
    bool aiEnabled = true,
  }) async {
    List<Map<String, Object>> foundQuestions = _findMatchingQuestions(message);

    if (foundQuestions.isNotEmpty) {
      return _generateResponseMessage(foundQuestions);
    }

    // Call the OpenAI API if no matching questions are found locally
    if (aiEnabled && OpenAI_API.oat().isNotEmpty) {
      newtworkOperation = await OpenAI_API.getResponseFromOpenAI(message,
          previousMessages: previousMessages);
      String response = await newtworkOperation!.value;
      String poweredTitle = OpenAI_API.model == 'gpt-4'
          ? 'Powered by GPT-4'
          : OpenAI_API.model == 'gpt-3.5-turbo'
              ? 'Powered by GPT-3.5'
              : OpenAI_API.model == 'huggingface'
                  ? HuggingFace_API.model()
                  : '';
      return [
        Message(
          text: response,
          createdAt: Timestamp.now(),
          userId: defaultUserId,
          username: _name == ''
              ? 'AI'
              : poweredTitle == ''
                  ? _name
                  : ('$_name ($poweredTitle)'),
        ),
      ];
    }

    return [];
  }

  double jaroSimilarity(String s1, String s2) {
    int maxDistance = (s1.length / 2).floor() - 1;

    List<bool> matches1 = List.filled(s1.length, false);
    List<bool> matches2 = List.filled(s2.length, false);

    int matches = 0;
    int transpositions = 0;

    for (int i = 0; i < s1.length; i++) {
      int start = max(0, i - maxDistance);
      int end = min(i + maxDistance + 1, s2.length);

      for (int j = start; j < end; j++) {
        if (matches2[j]) continue;
        if (s1[i] != s2[j]) continue;
        matches1[i] = true;
        matches2[j] = true;
        matches++;
        break;
      }
    }

    if (matches == 0) return 0.0;

    int k = 0;
    for (int i = 0; i < s1.length; i++) {
      if (!matches1[i]) continue;
      while (!matches2[k]) k++;
      if (s1[i] != s2[k]) transpositions++;
      k++;
    }

    double m = matches.toDouble();
    return (m / s1.length + m / s2.length + (m - transpositions / 2) / m) / 3;
  }

  double jaroWinkler(String s1, String s2, {double p = 0.1}) {
    double jaro = jaroSimilarity(s1, s2);
    int prefix = 0;
    for (int i = 0; i < min(s1.length, s2.length); i++) {
      if (s1[i] == s2[i]) {
        prefix++;
      } else {
        break;
      }
    }
    return jaro + prefix * p * (1 - jaro);
  }

  /// Searches the AI's question database for matching questions based on the user's input.
  ///
  /// [message] is the sanitized input message from the user.
  List<Map<String, Object>> _findMatchingQuestions(String message) {
    List<Map<String, Object>> foundQuestions = [];
    var userMessage = _sanitizeMessage(message);

    for (var questionMap in _questions!) {
      var question = _sanitizeMessage(questionMap['description'].toString());

      if (question == userMessage) {
        foundQuestions.add(questionMap);
        break; // Exit the loop early as we have found an exact match
      }
    }

    if (foundQuestions.isEmpty) {
      foundQuestions = _searchForQuestions(userMessage);
    }

    return foundQuestions;
  }

  /// Sanitizes the input message by removing special characters and converting it to lowercase.
  ///
  /// [message] is the raw input message.
  String _sanitizeMessage(String message) {
    return message.replaceAll('?', '').toLowerCase();
  }

  /// Searches the AI's question database for questions that contain the user's input message.
  ///
  /// [userMessage] is the sanitized input message from the user.
  List<Map<String, Object>> _searchForQuestions(String userMessage) {
    List<Map<String, Object>> foundQuestions = [];
    double similarityThreshold =
        0.98; // You can adjust this value to fine-tune the matching algorithm

    for (var questionMap in _questions!) {
      var question = _sanitizeMessage(questionMap['description'].toString());
      double similarity = jaroWinkler(userMessage, question);

      if (similarity >= similarityThreshold) {
        foundQuestions.add(questionMap);
      }
    }

    // Sort the found questions by their similarity in descending order
    foundQuestions.sort((a, b) {
      var aQuestion = _sanitizeMessage(a['description'].toString());
      var bQuestion = _sanitizeMessage(b['description'].toString());
      return jaroWinkler(userMessage, bQuestion)
          .compareTo(jaroWinkler(userMessage, aQuestion));
    });

    return foundQuestions;
  }

  /// Generates a response message based on the list of matching questions.
  ///
  /// [foundQuestions] is the list of questions that match the user's input message.
  Future<List<Message>> _generateResponseMessage(
      List<Map<String, Object>> foundQuestions) async {
    try {
      Random rnd = Random(DateTime.now().millisecondsSinceEpoch);
      List<String> messages = [];

      for (Map<String, Object> questionMap in foundQuestions) {
        messages.addAll(questionMap['buttonsTexts'] as Iterable<String>);
      }

      int index = rnd.nextInt(messages.length);

      return [
        Message(
          text: messages[index],
          createdAt: Timestamp.now(),
          userId: defaultUserId,
          username: _name == '' ? 'AI' : _name,
        ),
      ];
    } catch (e) {
      print('Error generating response message: $e');
    }
    return [];
  }
}
