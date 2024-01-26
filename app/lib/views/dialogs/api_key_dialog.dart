import 'package:flutter/material.dart';
import 'package:glowby/utils/utils.dart';

import '../../services/pulze_ai_api.dart';

class ApiKeyDialog extends StatefulWidget {
  const ApiKeyDialog({super.key});

  @override
  ApiKeyDialogState createState() => ApiKeyDialogState();
}

class ApiKeyDialogState extends State<ApiKeyDialog> {
  static const openAIKeyPattern = r'^sk-[A-Za-z0-9-_]+$';
  static const huggingFaceKeyPattern = r'^[A-Za-z0-9-_]+$';
  static const pulzeAIKeyPattern = r'^sk-[A-Za-z0-9-_]+$';

  final _apiKeyController = TextEditingController();
  final _pulzeAiController = TextEditingController();
  String _apiKey = '';
  String _pulzeAiToken = '';

  @override
  void initState() {
    super.initState();

    PulzeAiApi.loadOat().then((_) {
      setState(() {
        _apiKeyController.text = _apiKey;
        _pulzeAiToken = PulzeAiApi.oat();
        _apiKey = _pulzeAiToken;
        _pulzeAiController.text = _pulzeAiToken;
      });
    });
  }

  bool isValidOpenAIKey(String key) => RegExp(openAIKeyPattern).hasMatch(key);

  bool isValidHuggingFaceKey(String key) =>
      RegExp(huggingFaceKeyPattern).hasMatch(key);

  bool isValidPuzzleAIKey(String key) =>
      RegExp(pulzeAIKeyPattern).hasMatch(key);

  void _saveApiKey(BuildContext context) {
    String? errorMessage;

    if (_apiKey.isNotEmpty && !isValidOpenAIKey(_apiKey)) {
      errorMessage = 'OpenAI API Key is invalid!';
    } else if (_pulzeAiToken.isNotEmpty && !isValidPuzzleAIKey(_pulzeAiToken)) {
      errorMessage = 'Pulze API Key is invalid!';
    }

    if (errorMessage != null) {
      // If there's an error, show the error message and exit the function after popping the dialog.
      Navigator.pop(context); // Hide the dialog
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(errorMessage)),
      );
    } else {
      // If all keys are valid, set them and show a success message.
      PulzeAiApi.setOat(_pulzeAiToken);
      Navigator.pop(context); // Hide the dialog
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('API Keys saved successfully!')),
      );
    }
  }

  bool _obscureApiKeyPulze = true;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Enter Pulze API Key'),
      content: SizedBox(
        width: 340, // Set the max width of the AlertDialog
        child: SingleChildScrollView(
          child: ListBody(
            children: <Widget>[
              Align(
                alignment: Alignment.centerLeft,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const SizedBox(height: 10),
                    const Text('Get your Access Token:'),
                    InkWell(
                      child: const Text(
                        '→ Pulze Dashboard',
                        style: TextStyle(color: Colors.blue),
                      ),
                      onTap: () =>
                          Utils.launchURL('https://platform.pulze.ai/'),
                    ),
                    const SizedBox(height: 10),
                    const Text('Enter your Pulze Token:'),
                    TextField(
                      controller: _pulzeAiController,
                      obscureText: _obscureApiKeyPulze,
                      decoration: InputDecoration(
                        labelText: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
                        suffixIcon: IconButton(
                          icon: Icon(
                            // Change the icon based on whether the text is obscured
                            _obscureApiKeyPulze
                                ? Icons.visibility_off
                                : Icons.visibility,
                          ),
                          onPressed: () {
                            // Update the state to toggle the obscure text value
                            setState(() {
                              _obscureApiKeyPulze = !_obscureApiKeyPulze;
                            });
                          },
                        ),
                      ),
                      onChanged: (value) {
                        setState(() {
                          _pulzeAiToken = value;
                        });
                      },
                    ),
                    const SizedBox(height: 20),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          child: const Text('Clear'),
          onPressed: () {
            setState(() {
              _apiKeyController.clear();
              _apiKey = '';
              _pulzeAiController.clear();
              _pulzeAiToken = '';
            });
          },
        ),
        TextButton(
          child: const Text('Cancel'),
          onPressed: () {
            Navigator.pop(context);
          },
        ),
        ElevatedButton(
          child: const Text(
            'Save API Key',
            style: TextStyle(color: Colors.white),
          ),
          onPressed: () => _saveApiKey(context),
        ),
      ],
    );
  }
}
