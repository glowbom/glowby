import 'dart:math';

import 'timestamp.dart';
import 'message.dart';

/// A class representing the AI chatbot that processes and responds to user messages.
class Ai {
  final List<Map<String, Object>>? _questions;
  final String? _name;

  static const String defaultUserId = '007';

  Ai(this._name, this._questions);

  /// Processes the user's message and returns an AI-generated response.
  ///
  /// [message] is the input message from the user.
  Future<List<Message>> message(String message) async {
    List<Map<String, Object>> foundQuestions = _findMatchingQuestions(message);

    if (foundQuestions.isNotEmpty) {
      return _generateResponseMessage(foundQuestions);
    }

    return [];
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

    for (var questionMap in _questions!) {
      var question = _sanitizeMessage(questionMap['description'].toString());

      if (userMessage.contains(question)) {
        foundQuestions.add(questionMap);
      }
    }

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
