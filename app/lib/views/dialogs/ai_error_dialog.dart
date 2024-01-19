import 'package:flutter/material.dart';
import 'package:glowby/utils/utils.dart';

class AiErrorDialog extends StatefulWidget {
  const AiErrorDialog({super.key});

  @override
  AiErrorDialogState createState() => AiErrorDialogState();
}

class AiErrorDialogState extends State<AiErrorDialog> {
  @override
  void initState() {
    super.initState();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Something went wrong!'),
      content: SizedBox(
        width: 340, // Set the max width of the AlertDialog
        child: SingleChildScrollView(
          child: ListBody(
            children: <Widget>[
              const Text(
                  'Make sure you have access to the latest models. For API accounts created after August 18, 2023, you can get instant access to the latest models after purchasing \$0.50 worth or more of pre-paid credits.'),
              InkWell(
                child: const Text(
                  '→ More details',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => Utils.launchURL(
                    'https://help.openai.com/en/articles/8555510-gpt-4-turbo'),
              ),
              const SizedBox(height: 10),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          child: const Text('Ok'),
          onPressed: () {
            Navigator.pop(context);
          },
        ),
      ],
    );
  }
}
