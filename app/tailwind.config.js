/** @type {import('tailwindcss').Config} */
module.exports = {
  presets: [require("nativewind/preset")],
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./modules/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: "#f24f2d",
          soft: "#fff1ec",
          deep: "#b72f12"
        },
        ink: "#171717",
        muted: "#6b7280",
        panel: "#ffffff",
        border: "#e5e7eb",
        success: "#15803d",
        warning: "#c2410c",
        danger: "#b91c1c"
      }
    }
  },
  plugins: []
};
