import 'dart:math';

import 'timestamp.dart';
import 'message.dart';

class Ai {
  final List<Map<String, Object>>? _questions;
  final String? _name;

  static const String defaultUserId = '007';

  Ai(this._name, this._questions);

  Future<List<Message>> message(String message) async {
    List<Map<String, Object>> foundQuestions = _findMatchingQuestions(message);

    if (foundQuestions.isNotEmpty) {
      return _generateResponseMessage(foundQuestions);
    }

    return [];
  }

  List<Map<String, Object>> _findMatchingQuestions(String message) {
    List<Map<String, Object>> foundQuestions = [];
    var userMessage = _sanitizeMessage(message);

    for (var questionMap in _questions!) {
      var question = _sanitizeMessage(questionMap['description'].toString());

      if (question == userMessage) {
        foundQuestions.add(questionMap);
      }
    }

    if (foundQuestions.isEmpty) {
      foundQuestions = _searchForQuestions(userMessage);
    }

    return foundQuestions;
  }

  String _sanitizeMessage(String message) {
    return message.replaceAll('?', '').toLowerCase();
  }

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
      // Log the exception or handle it appropriately
    }

    return [];
  }
}
