import 'dart:math';
import 'package:flutter/material.dart';

/// Adjusts a color value by increasing its brightness towards white based on a given factor.
///
/// This function modifies a single RGB component (red, green, or blue) by applying a tinting factor.
/// It lightens the component by a percentage that moves it closer to 255 (white). The factor
/// should be between 0.0 (no change) and 1.0 (full white).
///
/// - `value`: An integer representing the color value of a single RGB component.
/// - `factor`: A double representing the factor by which the color value is to be increased.
///
/// Returns an integer representing the new color value after applying the tint factor, ensuring
/// it is within the valid range of 0 to 255.
int tintValue(int value, double factor) =>
    max(0, min((value + ((255 - value) * factor)).round(), 255));

/// Lightens a given color by mixing it with white based on a provided factor.
///
/// The `tintColor` function takes an original color and a factor, then computes
/// a lighter version of the original color. The factor determines how much white
/// is mixed with the original color, with 1.0 being fully white and 0 having no effect.
///
/// - `color`: The `Color` object that is to be tinted.
/// - `factor`: A double value between 0.0 and 1.0 that represents the intensity of the tint.
///             The closer the factor is to 1.0, the lighter the tint will be.
///
/// Returns a new `Color` object representing the tinted color.
Color tintColor(Color color, double factor) => Color.fromRGBO(
    tintValue(color.red, factor),
    tintValue(color.green, factor),
    tintValue(color.blue, factor),
    1);

/// Calculates a shaded color value by decreasing its luminance.
///
/// This function takes a color value (0-255) and a factor (0.0-1.0) and darkens
/// the color by the factor provided. The factor represents the percentage of
/// change towards black, with 1.0 being completely black.
///
/// - `value`: The base color value to be shaded.
/// - `factor`: The factor by which the color is to be darkened. A factor of 0.1
///   darkens the color by 10%, for example.
///
/// Returns an integer representing the shaded color value, ensuring it remains
/// within the valid color value range of 0 to 255.
int shadeValue(int value, double factor) =>
    max(0, min(value - (value * factor).round(), 255));

Color shadeColor(Color color, double factor) => Color.fromRGBO(
    shadeValue(color.red, factor),
    shadeValue(color.green, factor),
    shadeValue(color.blue, factor),
    1);

/// Generates a `MaterialColor` based on a single `Color`.
///
/// This function takes a `Color` and a luminance factor to produce different shades.
/// It creates a `MaterialColor` which allows for color consistency across different UI elements.
///
/// The shades are generated by tinting (lightening) and shading (darkening) the base color.
/// Tints are produced by adding the factor to the base color, moving towards white.
/// Shades are produced by subtracting the factor from the base color, moving towards black.
///
/// Each tint or shade is defined for the material design color swatch.
///
/// - `color`: The base color from which the swatch will be generated.
///
/// Returns a `MaterialColor` object allowing for consistent color theming.
MaterialColor generateMaterialColor(Color color) {
  return MaterialColor(color.value, {
    50: tintColor(color, 0.9),
    100: tintColor(color, 0.8),
    200: tintColor(color, 0.6),
    300: tintColor(color, 0.4),
    400: tintColor(color, 0.2),
    500: color,
    600: shadeColor(color, 0.1),
    700: shadeColor(color, 0.2),
    800: shadeColor(color, 0.3),
    900: shadeColor(color, 0.4),
  });
}
