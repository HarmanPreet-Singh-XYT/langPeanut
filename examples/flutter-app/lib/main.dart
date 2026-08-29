import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: AppLocalizations.of(context)!.flightpeanutMobile,
      home: const HomeScreen(),
    );
  }
}

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title:  Text(AppLocalizations.of(context)!.mainDashboard),
      ),
      body: Center(
        child: Column(
          children:  [
            Text(AppLocalizations.of(context)!.mainWelcomeback),
            Tooltip(message: AppLocalizations.of(context)!.mainViewsettings),
          ],
        ),
      ),
    );
  }
}
