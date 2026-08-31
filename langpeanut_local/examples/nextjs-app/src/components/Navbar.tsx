import React from 'react';

export interface NavbarProps {
  user?: { name: string; email: string };
  cartCount: number;
  onOpenCart: () => void;
}

export const Navbar: React.FC<NavbarProps> = ({ user, cartCount, onOpenCart }) => {
  return (
    <header className="navbar-container">
      <div className="brand-logo">
        <h1>FlightPeanut Store</h1>
      </div>
      <nav className="nav-links">
        <a href="/flights">Flights</a>
        <a href="/hotels">Hotels</a>
        <a href="/deals">Deals</a>
      </nav>
      <div className="nav-actions">
        <button onClick={onOpenCart} title="View your shopping cart">
          Cart ({cartCount})
        </button>
        {user ? (
          <div className="user-profile">
            <span>Welcome back, {user.name}!</span>
            <button onClick={() => console.log('LOGOUT_TRIGGERED')}>Sign Out</button>
          </div>
        ) : (
          <button onClick={() => console.log('LOGIN_TRIGGERED')}>Sign In</button>
        )}
      </div>
    </header>
  );
};
