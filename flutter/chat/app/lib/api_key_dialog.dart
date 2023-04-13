import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher_string.dart';

class ApiKeyDialog extends StatefulWidget {
  @override
  _ApiKeyDialogState createState() => _ApiKeyDialogState();
}

class _ApiKeyDialogState extends State<ApiKeyDialog> {
  final _apiKeyController = TextEditingController();
  String _apiKey = '';

  void _saveApiKey() {
    // Save the API key locally and validate it
  }

  void _launchURL(String url) async {
    if (await canLaunchUrlString(url)) {
      await launchUrlString(url);
    } else {
      throw 'Could not launch $url';
    }
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
                onTap: () =>
                    _launchURL('https://platform.openai.com/account/api-keys'),
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
              SizedBox(height: 10),
              ElevatedButton(
                child: Text('Save API Key'),
                onPressed: _saveApiKey,
              ),
              SizedBox(height: 20),
              InkWell(
                child: Text(
                  'API Key not working? Click Here.',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () => _launchURL(
                    'https://platform.openai.com/account/billing/overview'),
              ),
              Text('Ensure billing info is added in OpenAI Billing.'),
              SizedBox(height: 20),
              InkWell(
                child: Text(
                  'The Price is about 100,000 words per \$1',
                  style: TextStyle(color: Colors.blue),
                ),
                onTap: () =>
                    _launchURL('https://openai.com/pricing#language-models'),
              ),
              Text('ChatGPT Plus subscription not required.'),
            ],
          ),
        ),
      ),
    );
  }
}
