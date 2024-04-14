import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class MultiOnApi {
  static final MultiOnApi _instance = MultiOnApi._privateConstructor();
  factory MultiOnApi() => _instance;
  MultiOnApi._privateConstructor();

  String _apiKey = '';

  static String oat() => MultiOnApi()._oat();
  static void setOat(String value) => MultiOnApi()._setOat(value);
  static void resetOat() => MultiOnApi()._resetOat();

  void _resetOat() {
    _apiKey = '';
  }

  String _oat() => _apiKey;
  Future<void> _setOat(String value) async {
    _apiKey = value;
    await _secureStorage.write(key: _apiKeyKey, value: _apiKey);
  }

  static const String _apiKeyKey = 'multion_api_key';
  static const FlutterSecureStorage _secureStorage = FlutterSecureStorage();

  static Future<void> loadOat() async {
    try {
      setOat(await _secureStorage.read(key: _apiKeyKey) ?? '');
    } catch (e) {
      if (kDebugMode) {
        print('Error loading OAT: $e');
      }
    }
  }
}
