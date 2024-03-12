import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:flutter_highlight/flutter_highlight.dart';
import 'package:flutter_highlight/themes/github.dart';
import 'package:flutter/services.dart';

class MessageBubble extends StatelessWidget {
  final String message;
  final bool isMe;
  final String? username;
  final String? link;

  const MessageBubble(this.message, this.username, this.isMe, this.link,
      {super.key});

  // Launches the link if it is valid
  void _launchLink({String l = ""}) async {
    final uri = Uri.parse(l == "" ? link! : l);
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri);
    } else {
      throw Exception('Could not launch $link');
    }
  }

  // Builds the message bubble container with the appropriate decoration
  Container _buildMessageBubbleContainer(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        // Different colors for sender and receiver
        color: isMe ? Colors.grey[300] : Theme.of(context).primaryColor,
        borderRadius: BorderRadius.only(
          topLeft: const Radius.circular(12),
          topRight: const Radius.circular(12),
          bottomLeft: Radius.circular(isMe ? 12 : 0),
          bottomRight: Radius.circular(isMe ? 0 : 12),
        ),
      ),
      width: 280,
      padding: const EdgeInsets.symmetric(
        vertical: 10,
        horizontal: 16,
      ),
      margin: const EdgeInsets.symmetric(
        vertical: 4,
        horizontal: 8,
      ),
      child: _buildMessageContent(context),
    );
  }

  // Builds the message content, including the username, message, and link (if available)
  // CrossAxisAlignment is determined based on the sender (isMe)
  Column _buildMessageContent(BuildContext context) {
    return Column(
      crossAxisAlignment:
          isMe ? CrossAxisAlignment.end : CrossAxisAlignment.start,
      children: [
        _buildUsernameText(context),
        _buildMessageOrLink(context),
      ],
    );
  }

  // Builds the username text with bold font weight and appropriate color
  Text _buildUsernameText(BuildContext context) {
    return Text(
      username!,
      style: TextStyle(
        fontWeight: FontWeight.bold,
        color: isMe ? Colors.black : Colors.white70,
      ),
    );
  }

  // This method checks if the message indicates an image should be displayed.
  // The message 'image' is a special keyword in our system that tells the app to render an image from the provided link.
  // This convention is used to differentiate between messages that are text and ones that should display an image.
  // The link is expected to be a direct URL to an image resource.
  // If the link does not result in a valid image, the image widget will handle the error and may fall back to a placeholder or error widget.
  Widget _buildMessageOrLink(BuildContext context) {
    if (link == null) {
      if (message.contains('```')) {
        // Split the message by the code block
        final parts = message.split('```');
        final normalText = parts[0];
        String codeBlock = parts.length > 1 ? parts[1] : '';
        String language = 'plaintext';

        // Check if there's a specified language
        final codeParts = codeBlock.split('\n');
        if (codeParts.length > 1 && codeParts[0].trim().isNotEmpty) {
          language = codeParts[0].trim(); // The first line is the language
          codeBlock = codeParts.sublist(1).join('\n'); // The rest is the code
        }

        return Column(
          crossAxisAlignment:
              isMe ? CrossAxisAlignment.end : CrossAxisAlignment.start,
          children: [
            // Display normal text as selectable
            SelectableText(
              normalText,
              style: TextStyle(color: isMe ? Colors.black : Colors.white70),
              textAlign: isMe ? TextAlign.end : TextAlign.start,
            ),
            // Display code block with syntax highlighting
            if (codeBlock.isNotEmpty)
              Container(
                padding: const EdgeInsets.all(8.0),
                color: Colors
                    .grey[200], // Light grey background for the code block
                child: Column(
                  children: [
                    HighlightView(
                      codeBlock
                          .trim(), // Trim the code block to remove leading/trailing whitespace
                      language: language, // Specify the language
                      theme:
                          githubTheme, // Specify the theme for syntax highlighting
                      textStyle: const TextStyle(
                          fontFamily:
                              'monospace'), // Optional: specify text style
                    ),
                    // Copy Code button
                    TextButton(
                      onPressed: () {
                        Clipboard.setData(
                            ClipboardData(text: codeBlock.trim()));
                        // Optionally, show a snackbar or toast to indicate that the code has been copied
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(
                              content: Text('Code copied to clipboard!')),
                        );
                      },
                      child: const Text('Copy Code'),
                    ),
                  ],
                ),
              ),
            // Display the rest of the message as selectable, if any
            if (parts.length > 2)
              SelectableText(
                parts.sublist(2).join('```'),
                style: TextStyle(color: isMe ? Colors.black : Colors.white70),
                textAlign: isMe ? TextAlign.end : TextAlign.start,
              ),
          ],
        );
      } else {
        return _buildMessageText(context);
      }
    } else if (message == 'image') {
      return Image.network(
        link!,
        errorBuilder: (context, error, stackTrace) {
          // Fallback for when the image fails to load
          return _buildLinkButton(context);
        },
      );
    } else {
      return _buildLinkButton(context);
    }
  }

  // Builds the message text with the appropriate color and alignment
  Widget _buildMessageText(BuildContext context) {
    // Split the message by '**' to identify bold sections
    final parts = message.split('**');
    List<TextSpan> spans = [];

    // Regular expression to match URLs
    final urlRegex = RegExp(r'https?:\/\/[^\s)]+', caseSensitive: false);

    // Iterate over the parts and apply bold style to every second element
    for (int i = 0; i < parts.length; i++) {
      final part = parts[i];
      // Check if the part contains a URL
      if (urlRegex.hasMatch(part)) {
        final matches = urlRegex.allMatches(part);
        int lastMatchEnd = 0;

        for (var match in matches) {
          // Add text before the URL
          spans.add(TextSpan(
            text: part.substring(lastMatchEnd, match.start),
            style: TextStyle(
              color: isMe ? Colors.black : Colors.white70,
              fontWeight: i % 2 == 1 ? FontWeight.bold : FontWeight.normal,
            ),
          ));

          // Add the URL with a link style and gesture recognizer
          spans.add(TextSpan(
            text: part.substring(match.start, match.end),
            style: const TextStyle(
              color: Colors.blue,
              decoration: TextDecoration.underline,
            ),
            recognizer: TapGestureRecognizer()
              ..onTap = () {
                _launchLink(l: part.substring(match.start, match.end));
              },
          ));

          lastMatchEnd = match.end;
        }

        // Add any remaining text after the last URL
        if (lastMatchEnd < part.length) {
          spans.add(TextSpan(
            text: part.substring(lastMatchEnd),
            style: TextStyle(
              color: isMe ? Colors.black : Colors.white70,
              fontWeight: i % 2 == 1 ? FontWeight.bold : FontWeight.normal,
            ),
          ));
        }
      } else {
        spans.add(TextSpan(
          text: part,
          style: TextStyle(
            color: isMe ? Colors.black : Colors.white70,
            fontWeight: i % 2 == 1 ? FontWeight.bold : FontWeight.normal,
          ),
        ));
      }
    }

    return SelectableText.rich(
      TextSpan(children: spans),
      textAlign: TextAlign.start,
    );
  }

// Existing _launchLink method can be used here

  // Builds the link button, which launches the link when pressed
  ElevatedButton _buildLinkButton(BuildContext context) {
    return ElevatedButton(
      style: ElevatedButton.styleFrom(
        backgroundColor: Colors.blue, // Background color
      ),
      onPressed: _launchLink,
      child: Text(
        message,
        textAlign: isMe ? TextAlign.end : TextAlign.start,
        style: TextStyle(color: isMe ? Colors.black : Colors.white70),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    // Aligns the message bubble to the right (sender) or left (receiver) side of the screen
    return Row(
      mainAxisAlignment: isMe ? MainAxisAlignment.end : MainAxisAlignment.start,
      children: <Widget>[
        _buildMessageBubbleContainer(context),
      ],
    );
  }
}
