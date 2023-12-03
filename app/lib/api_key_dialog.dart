import 'package:flutter/material.dart';
import 'package:glowby/openai_api.dart';
import 'package:glowby/utils.dart';

import 'hugging_face_api.dart';
import 'pulze_ai_api.dart';

class ApiKeyDialog extends StatefulWidget {
  @override
  _ApiKeyDialogState createState() => _ApiKeyDialogState();
}

class _ApiKeyDialogState extends State<ApiKeyDialog> {
  final _apiKeyController = TextEditingController();
  final _huggingFaceTokenController = TextEditingController();
  final _pulzeAiController = TextEditingController();
  String _apiKey = '';
  String _huggingFaceToken = '';
  String _pulzeAiToken = '';

  @override
  void initState() {
    super.initState();

    OpenAI_API.loadOat().then((_) {
      setState(() {
        _apiKey = OpenAI_API.oat();
        _apiKeyController.text = _apiKey;
        _huggingFaceToken = HuggingFace_API.oat();
        _huggingFaceTokenController.text = _huggingFaceToken;
        _pulzeAiToken = PulzeAI_API.oat();
        _pulzeAiController.text = _pulzeAiToken;
      });
    });
  }

  void _saveApiKey(BuildContext context) {
    OpenAI_API.setOat(_apiKey);
    HuggingFace_API.setOat(_huggingFaceToken);
    PulzeAI_API.setOat(_pulzeAiToken);
    Navigator.pop(context); // Hide the dialog

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('API Key saved successfully!')),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text('Enter OpenAI API Key'),
      content: Container(
        width: 340, // Set the max width of the AlertDialog
        child: SingleChildScrollView(
          child: ListBody(
            children: <Widget>[
              Text('Get your API key:'),
              InkWell(
                child: Text(
                  '→ OpenAI Dashboard',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => Utils.launchURL(
                    'https://platform.openai.com/account/api-keys'),
              ),
              SizedBox(height: 10),
              Text('API Key is stored locally and not shared.'),
              TextField(
                controller: _apiKeyController,
                obscureText: true,
                decoration: InputDecoration(
                    labelText: 'sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'),
                onChanged: (value) {
                  setState(() {
                    _apiKey = value;
                  });
                },
              ),
              SizedBox(height: 20),
              InkWell(
                child: Text(
                  'API Key not working? Click Here.',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => Utils.launchURL(
                    'https://platform.openai.com/account/billing/overview'),
              ),
              Text('Ensure billing info is added in OpenAI Billing.'),
              SizedBox(height: 10),
              InkWell(
                child: Text(
                  'The Price is about 100,000 words per \$1',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => Utils.launchURL(
                    'https://openai.com/pricing#language-models'),
              ),
              Text('ChatGPT Plus subscription not required.'),
              SizedBox(height: 10),
              Divider(),
              ExpansionTile(
                title: Text(
                  'Pulze.ai',
                  style: TextStyle(fontWeight: FontWeight.bold),
                ),
                children: <Widget>[
                  Align(
                    alignment: Alignment.centerLeft,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        SizedBox(height: 10),
                        Text('Get your Access Token:'),
                        InkWell(
                          child: Text(
                            '→ Pulze.ai Dashboard',
                            style: TextStyle(color: Colors.blue),
                          ),
                          onTap: () => Utils.launchURL(
                              'https://platform.pulze.ai/'),
                        ),
                        SizedBox(height: 10),
                        Text('Enter your Pulze.ai Token:'),
                        TextField(
                          controller: _pulzeAiController,
                          obscureText: true,
                          decoration: InputDecoration(
                              labelText: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'),
                          onChanged: (value) {
                            setState(() {
                              _pulzeAiToken = value;
                            });
                          },
                        ),
                        SizedBox(height: 20),
                      ],
                    ),
                  ),
                ],
              ),
              SizedBox(height: 10),
              Divider(),
              SizedBox(height: 10),
              ExpansionTile(
                title: Text(
                  '🤗 Hosted Inference API',
                  style: TextStyle(fontWeight: FontWeight.bold),
                ),
                children: <Widget>[
                  Align(
                    alignment: Alignment.centerLeft,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        SizedBox(height: 10),
                        Text('Get your Access Token:'),
                        InkWell(
                          child: Text(
                            '→ Hugging Face Dashboard',
                            style: TextStyle(color: Colors.blue),
                          ),
                          onTap: () => Utils.launchURL(
                              'https://huggingface.co/settings/tokens'),
                        ),
                        SizedBox(height: 10),
                        Text('Enter your Hugging Face Token:'),
                        TextField(
                          controller: _huggingFaceTokenController,
                          obscureText: true,
                          decoration: InputDecoration(
                              labelText: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'),
                          onChanged: (value) {
                            setState(() {
                              _huggingFaceToken = value;
                            });
                          },
                        ),
                        SizedBox(height: 20),
                      ],
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          child: Text('Clear'),
          onPressed: () {
            setState(() {
              _apiKeyController.clear();
              _apiKey = '';
              _huggingFaceTokenController.clear();
              _huggingFaceToken = '';
              _pulzeAiController.clear();
              _pulzeAiToken = '';
            });
          },
        ),
        TextButton(
          child: Text('Cancel'),
          onPressed: () {
            Navigator.pop(context);
          },
        ),
        ElevatedButton(
          child: Text(
            'Save API Key',
            style: TextStyle(color: Colors.white),
          ),
          onPressed: () => _saveApiKey(context),
        ),
      ],
    );
  }
}
