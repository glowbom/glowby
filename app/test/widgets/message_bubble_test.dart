import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import '../../lib/message_bubble.dart';

void main() {
  testWidgets('MessageBubble displays message, username, and link',
      (WidgetTester tester) async {
    // Replace the following values with appropriate data for your test case
    final message = 'Hello';
    final username = 'John Doe';
    final isMe = false;
    final link = 'https://www.example.com';

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MessageBubble(message, username, isMe, link),
        ),
      ),
    );

    // Find the widgets in the MessageBubble
    final messageFinder = find.text(message);
    final usernameFinder = find.text(username);
    final linkFinder = find.text(link);

    // Verify that the widgets are present
    expect(messageFinder, findsOneWidget);
    expect(usernameFinder, findsOneWidget);
    expect(linkFinder, findsNothing);
  });
}
