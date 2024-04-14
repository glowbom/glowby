import 'package:flutter/material.dart';
import 'package:glowby/services/mution_api.dart';
import 'package:glowby/services/openai_api.dart';
import 'package:glowby/utils/utils.dart';

import '../../services/hugging_face_api.dart';
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
  final _apiKeyMultiOnController = TextEditingController();
  final _huggingFaceTokenController = TextEditingController();
  final _pulzeAiController = TextEditingController();
  String _apiKey = '';
  String _apiKeyMultiOn = '';
  String _huggingFaceToken = '';
  String _pulzeAiToken = '';

  @override
  void initState() {
    super.initState();

    MultiOnApi.loadOat().then((_) {
      setState(() {
        _apiKeyMultiOn = MultiOnApi.oat();
        _apiKey = OpenAiApi.oat();
        _apiKeyController.text = _apiKey;
        _huggingFaceToken = HuggingFaceApi.oat();
        _huggingFaceTokenController.text = _huggingFaceToken;
        _pulzeAiToken = PulzeAiApi.oat();
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
    } else if (_huggingFaceToken.isNotEmpty &&
        !isValidHuggingFaceKey(_huggingFaceToken)) {
      errorMessage = 'Hugging Face Token is invalid!';
    } else if (_pulzeAiToken.isNotEmpty && !isValidPuzzleAIKey(_pulzeAiToken)) {
      errorMessage = 'Pulze API Key is invalid!';
    } else if (_apiKeyMultiOn.isEmpty) {
      errorMessage = 'Enter MultiOn API Key';
    }

    if (errorMessage != null) {
      // If there's an error, show the error message and exit the function after popping the dialog.
      Navigator.pop(context); // Hide the dialog
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(errorMessage)),
      );
    } else {
      // If all keys are valid, set them and show a success message.
      OpenAiApi.setOat(_apiKey);
      MultiOnApi.setOat(_apiKeyMultiOn);
      HuggingFaceApi.setOat(_huggingFaceToken);
      PulzeAiApi.setOat(_pulzeAiToken);
      Navigator.pop(context); // Hide the dialog
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('API Keys saved successfully!')),
      );
    }
  }

  bool _obscureApiKey = true;
  bool _obscureApiKeyPulze = true;
  bool _obscureApiKeyHuggingFace = true;
  bool _obscureApiKeyMultiOn = true;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Enter MultiOn API Key'),
      //title: const Text('Enter OpenAI API Key'),
      content: SizedBox(
        width: 340, // Set the max width of the AlertDialog
        child: SingleChildScrollView(
          child: ListBody(
            children: <Widget>[
              const Text('Get your API key:'),
              InkWell(
                child: const Text(
                  '→ MultiOn Dashboard',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => Utils.launchURL('https://app.multion.ai/api-keys'),
              ),
              const SizedBox(height: 10),
              const Text('API Key is stored locally and not shared.'),
              TextField(
                controller: _apiKeyMultiOnController,
                obscureText: _obscureApiKeyMultiOn,
                decoration: InputDecoration(
                  labelText: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
                  suffixIcon: IconButton(
                    icon: Icon(
                      // Change the icon based on whether the text is obscured
                      _obscureApiKeyMultiOn
                          ? Icons.visibility_off
                          : Icons.visibility,
                    ),
                    onPressed: () {
                      // Update the state to toggle the obscure text value
                      setState(() {
                        _obscureApiKeyMultiOn = !_obscureApiKeyMultiOn;
                      });
                    },
                  ),
                ),
                onChanged: (value) {
                  setState(() {
                    _apiKeyMultiOn = value;
                  });
                },
              ),
              const SizedBox(height: 20),
              /*const Text('Get your API key:'),
              InkWell(
                child: const Text(
                  '→ OpenAI Dashboard',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => Utils.launchURL(
                    'https://platform.openai.com/account/api-keys'),
              ),
              const SizedBox(height: 10),
              const Text('API Key is stored locally and not shared.'),
              TextField(
                controller: _apiKeyController,
                obscureText: _obscureApiKey,
                decoration: InputDecoration(
                  labelText: 'sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
                  suffixIcon: IconButton(
                    icon: Icon(
                      // Change the icon based on whether the text is obscured
                      _obscureApiKey ? Icons.visibility_off : Icons.visibility,
                    ),
                    onPressed: () {
                      // Update the state to toggle the obscure text value
                      setState(() {
                        _obscureApiKey = !_obscureApiKey;
                      });
                    },
                  ),
                ),
                onChanged: (value) {
                  setState(() {
                    _apiKey = value;
                  });
                },
              ),
              const SizedBox(height: 20),
              InkWell(
                child: const Text(
                  'API Key not working? Click Here.',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => Utils.launchURL(
                    'https://platform.openai.com/account/billing/overview'),
              ),
              const Text('Ensure billing info is added in OpenAI Billing.'),
              const SizedBox(height: 10),
              InkWell(
                child: const Text(
                  'The Price is about 100,000 words per \$1',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => Utils.launchURL(
                    'https://openai.com/pricing#language-models'),
              ),
              const Text('ChatGPT Plus subscription not required.'),
              const SizedBox(height: 10),
              const Divider(),
              ExpansionTile(
                title: const Text(
                  'Pulze',
                  style: TextStyle(fontWeight: FontWeight.bold),
                ),
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
              const SizedBox(height: 10),
              const Divider(),
              const SizedBox(height: 10),
              ExpansionTile(
                title: const Text(
                  '🤗 Hosted Inference API',
                  style: TextStyle(fontWeight: FontWeight.bold),
                ),
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
                            '→ Hugging Face Dashboard',
                            style: TextStyle(color: Colors.blue),
                          ),
                          onTap: () => Utils.launchURL(
                              'https://huggingface.co/settings/tokens'),
                        ),
                        const SizedBox(height: 10),
                        const Text('Enter your Hugging Face Token:'),
                        TextField(
                          controller: _huggingFaceTokenController,
                          obscureText: _obscureApiKeyHuggingFace,
                          decoration: InputDecoration(
                            labelText: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
                            suffixIcon: IconButton(
                              icon: Icon(
                                // Change the icon based on whether the text is obscured
                                _obscureApiKeyHuggingFace
                                    ? Icons.visibility_off
                                    : Icons.visibility,
                              ),
                              onPressed: () {
                                // Update the state to toggle the obscure text value
                                setState(() {
                                  _obscureApiKeyHuggingFace =
                                      !_obscureApiKeyHuggingFace;
                                });
                              },
                            ),
                          ),
                          onChanged: (value) {
                            setState(() {
                              _huggingFaceToken = value;
                            });
                          },
                        ),
                        const SizedBox(height: 20),
                      ],
                    ),
                  ),
                ],
              ),
              */
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
              _apiKeyMultiOnController.clear();
              _apiKeyMultiOn = '';
              _huggingFaceTokenController.clear();
              _huggingFaceToken = '';
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
