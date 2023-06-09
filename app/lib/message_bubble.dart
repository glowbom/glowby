import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

class MessageBubble extends StatelessWidget {
  final String message;
  final bool isMe;
  final Key? key;
  final String? username;
  final String? link;

  MessageBubble(this.message, this.username, this.isMe, this.link, {this.key});

  // Launches the link if it is valid
  void _launchLink() async {
    if (await canLaunchUrl(Uri.parse(link!))) {
      await launchUrl(Uri.parse(link!));
    } else {
      throw 'Could not launch $link';
    }
  }

  // Builds the message bubble container with the appropriate decoration
  Container _buildMessageBubbleContainer(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        // Different colors for sender and receiver
        color: isMe ? Colors.grey[300] : Theme.of(context).primaryColor,
        borderRadius: BorderRadius.only(
          topLeft: Radius.circular(12),
          topRight: Radius.circular(12),
          bottomLeft: Radius.circular(isMe ? 12 : 0),
          bottomRight: Radius.circular(isMe ? 0 : 12),
        ),
      ),
      width: 280,
      padding: EdgeInsets.symmetric(
        vertical: 10,
        horizontal: 16,
      ),
      margin: EdgeInsets.symmetric(
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

  // Builds the message or link, depending on the provided data
  // If a link is provided, it will display the link as a button or an image, depending on the message content
  Widget _buildMessageOrLink(BuildContext context) {
    if (link == null) {
      return _buildMessageText(context);
    } else if (message == 'image') {
      return Image.network(link!);
    } else {
      return _buildLinkButton(context);
    }
  }

  // Builds the message text with the appropriate color and alignment
  Widget _buildMessageText(BuildContext context) {
    return SelectableText.rich(
      TextSpan(
        children: [
          TextSpan(
            text: message,
            style: TextStyle(color: isMe ? Colors.black : Colors.white70),
          ),
          // Add more TextSpans if needed for different styles within the message
        ],
      ),
      textAlign: TextAlign.start,
    );
  }

  // Builds the link button, which launches the link when pressed
  ElevatedButton _buildLinkButton(BuildContext context) {
    return ElevatedButton(
      style: ElevatedButton.styleFrom(
        backgroundColor: Colors.blue, // Background color
      ),
      child: Text(
        message,
        textAlign: isMe ? TextAlign.end : TextAlign.start,
        style: TextStyle(color: isMe ? Colors.black : Colors.white70),
      ),
      onPressed: _launchLink,
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
