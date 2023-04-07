import 'dart:math';

import 'timestamp.dart';

import 'message.dart';

class Ai {
  final List<Map<String, Object>> _questions;
  final String _name;

  Ai(this._name, this._questions);

  Future<List<Message>> message(String message) async {
    List<Map<String, Object>> foundQuestions = [];

    var userMessage = message.replaceAll('?', '').toLowerCase();

    for (var q in _questions) {
      var question =
          q['description'].toString().replaceAll('?', '').toLowerCase();

      if (question == userMessage) {
        // exact match
        foundQuestions.add(q);
      }
    }

    if (foundQuestions.length == 0) {
      for (var q in _questions) {
        var question =
            q['description'].toString().replaceAll('?', '').toLowerCase();
        if (userMessage.contains('question')) {
          // good match
          foundQuestions.add(q);
        } else if (foundQuestions.length == 0) {
          // let's try to find intersection
          Set<String> userMessageSet = Set<String>();
          List<String> words = userMessage.split(' ');
          for (String word in words) {
            userMessageSet.add(word);
          }

          Set<String> questionSet = Set<String>();
          words = question.replaceAll("'s", "").split(' ');
          for (String word in words) {
            questionSet.add(word);
          }

          Set<String> intersection = userMessageSet.intersection(questionSet);
          double percentPerWord = 100 / words.length;
          double percentage = intersection.length * percentPerWord;

          if (percentage > 70) {
            foundQuestions.add(q);
          }
        }
      }
    }

    if (foundQuestions.length > 0) {
      try {
        Random rnd = new Random(DateTime.now().millisecondsSinceEpoch);
        List<String> messages = [];

        for (Map<String, Object> q in foundQuestions) {
          messages.addAll(q['buttonsTexts']);
        }

        int index = rnd.nextInt(messages.length);

        return [
          Message(
              text: messages[index],
              createdAt: Timestamp.now(),
              userId: '007',
              username: _name == '' ? 'AI' : _name),
        ];
      } catch (e) {}
    }

    return [];
  }
}
