// Importing required packages
import 'package:flutter/material.dart';
import 'views/screens/talk_screen.dart';

// Entry point of the application
// runApp starts the Flutter app by inflating the given widget and attaching it to the screen
void main() => runApp(const TalkApp());

/*class MyApp extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    String? link =
        'https://multion-client-screenshots.s3.us-east-2.amazonaws.com/74e23a1a-f9d5-4ad4-a559-595a78e3d666_1b3938ba-75ff-4fa6-829a-3c3516de057a_screenshot.png';
    return MaterialApp(
      home: Scaffold(
        body: Center(
          child: Image.network(
            link,
            errorBuilder: (context, error, stackTrace) {
              if (kDebugMode) {
                print('$link failed to load: $error');
              }
              return Image.network(
                  'https://glowbom.github.io/glowby-basic/images/image-not-found.jpg');
            },
          ),
        ),
      ),
    );
  }
}*/
