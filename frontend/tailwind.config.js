/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        accent: {
          DEFAULT: 'var(--color-accent)',
          hover: 'var(--color-accent-hover)',
          light: 'var(--color-accent-light)',
          glow: 'var(--color-accent-glow)',
        },
      },
      fontFamily: {
        sans: ['Inter', 'Vazirmatn', '-apple-system', 'sans-serif'],
        mono: ['JetBrains Mono', 'Consolas', 'monospace'],
        fa: ['Vazirmatn', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
