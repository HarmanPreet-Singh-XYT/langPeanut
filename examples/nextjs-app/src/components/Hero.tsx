import React from 'react';
import { useTranslation } from 'react-i18next';

export const Hero: React.FC = () => {
  return (
    <section className="hero-section">
      <div className="hero-content">
        <h2>{t('exploreTheWorldWithoutLanguage')}</h2>
        <p>{t('bookFlightsReserveLuxuryHotels')}</p>
        <div className="hero-cta-group">
          <button className="btn-primary">{t('reserveFlightBtn')}</button>
          <button className="btn-secondary">{t('heroSearch')}</button>
        </div>
      </div>
    </section>
  );
};
