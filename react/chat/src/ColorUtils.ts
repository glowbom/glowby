import Color from "color";

const tintValue = (value: number, factor: number): number =>
  Math.max(0, Math.min(Math.round(value + (255 - value) * factor), 255));

const tintColor = (color: Color, factor: number): Color =>
  color.rgb(
    tintValue(color.red(), factor),
    tintValue(color.green(), factor),
    tintValue(color.blue(), factor)
  );

const shadeValue = (value: number, factor: number): number =>
  Math.max(0, Math.min(value - Math.round(value * factor), 255));

const shadeColor = (color: Color, factor: number): Color =>
  color.rgb(
    shadeValue(color.red(), factor),
    shadeValue(color.green(), factor),
    shadeValue(color.blue(), factor)
  );

export const generateMaterialColor = (color: Color): Record<number, Color> => {
  return {
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
  };
};

