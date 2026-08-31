import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

// Automatically initialized by langPeanut for seamless react-i18next localization
if (!i18n.isInitialized) {
  i18n.use(initReactI18next).init({
    fallbackLng: 'en',
    interpolation: {
      escapeValue: false, // React already escapes values safely
    },
  });
}

export default i18n;
