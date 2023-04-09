import 'timestamp.dart';

class Message {
  final String text;
  final Timestamp createdAt;
  final String userId;
  final String? username;
  final String? link;

  Message({
    required this.text,
    required this.createdAt,
    required this.userId,
    required this.username,
    this.link,
  });
}
