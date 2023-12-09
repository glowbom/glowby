import 'timestamp.dart';

/// A class representing a chat message with a text, timestamp, user ID, and optional username and link.
class Message {
  /// The text content of the message.
  final String text;

  /// The timestamp when the message was created.
  final Timestamp createdAt;

  /// The user ID of the sender.
  final String userId;

  /// The username of the sender (optional).
  final String? username;

  /// A link associated with the message, if any.
  final String? link;

  Message({
    required this.text,
    required this.createdAt,
    required this.userId,
    this.username,
    this.link,
  });

  @override
  String toString() {
    return 'Message(text: $text, createdAt: $createdAt, userId: $userId, username: $username, link: $link)';
  }
}
