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
    List<String> parts = [
      'Message(text: $text',
      'createdAt: $createdAt',
      'userId: $userId',
    ];

    if (username != null) parts.add('username: $username');
    if (link != null) parts.add('link: $link');

    return parts.join(', ') + ')';
  }
}
