import React, { useState } from 'react';

export interface CartModalProps {
  isOpen: boolean;
  onClose: () => void;
  total: number;
}

export const CartModal: React.FC<CartModalProps> = ({ isOpen, onClose, total }) => {
  const [coupon, setCoupon] = useState('');

  if (!isOpen) return null;

  return (
    <div className="modal-backdrop">
      <div className="modal-card">
        <h3>Checkout Summary</h3>
        <div className="coupon-row">
          <input
            type="text"
            value={coupon}
            onChange={(e) => setCoupon(e.target.value)}
            placeholder="Enter discount code"
          />
          <button onClick={() => alert('Coupon Applied')}>Apply Coupon</button>
        </div>
        <div className="price-summary">
          <span>Total: ${total}</span>
        </div>
        <div className="modal-actions">
          <button onClick={onClose}>Cancel</button>
          <button onClick={() => alert('Order Placed')} className="btn-confirm">
            Submit Order
          </button>
        </div>
      </div>
    </div>
  );
};
