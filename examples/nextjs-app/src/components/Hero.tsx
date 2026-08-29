import React from 'react';

export const Hero: React.FC = () => {
  return (
    <section className="hero-section">
      <div className="hero-content">
        <h2>Explore the World Without Language Barriers</h2>
        <p>Book flights, reserve luxury hotels, and travel with real-time multi-agent translation.</p>
        <div className="hero-cta-group">
          <button className="btn-primary">Book</button>
          <button className="btn-secondary">Search</button>
        </div>
      </div>
    </section>
  );
};
