import React from 'react';
import { useTranslation } from 'react-i18next';

export interface NavbarProps {
  user?: { name: string; email: string };
  cartCount: number;
  onOpenCart: () => void;
}

export const Navbar: React.FC<NavbarProps> = ({ user, cartCount, onOpenCart }) => {
  return (
    <header className="navbar-container">
      <div className="brand-logo">
        <h1>{t('flightpeanutStore')}</h1>
      </div>
      <nav className="nav-links">
        <a href="/flights">{t('navbarFlights')}</a>
        <a href="/hotels">{t('navbarHotels')}</a>
        <a href="/deals">{t('navbarDeals')}</a>
      </nav>
      <div className="nav-actions">
        <button onClick={onOpenCart} title={t('titleViewYourShoppingCart')}>{t('navbarCart', { cartCount })}</button>
        {user ? (
          <div className="user-profile">
            <span>{t('navbarWelcomeback', { name })}</span>
            <button onClick={() => console.log('LOGOUT_TRIGGERED')}>{t('navbarSignout')}</button>
          </div>
        ) : (
          <button onClick={() => console.log('LOGIN_TRIGGERED')}>{t('navbarSignin')}</button>
        )}
      </div>
    </header>
  );
};
