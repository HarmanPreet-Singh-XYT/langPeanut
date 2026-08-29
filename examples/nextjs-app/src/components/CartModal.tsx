import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';

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
        <h3>{t('checkoutSummary')}</h3>
        <div className="coupon-row">
          <input
            type="text"
            value={coupon}
            onChange={(e) => setCoupon(e.target.value)}
            placeholder={t('placeholderEnterDiscountCode')}
          />
          <button onClick={() => alert('Coupon Applied')}>{t('cartmodalApplycoupon')}</button>
        </div>
        <div className="price-summary">
          <span>{t('cartmodalTotal', { total })}</span>
        </div>
        <div className="modal-actions">
          <button onClick={onClose}>{t('cartmodalCancel')}</button>
          <button onClick={() => alert('Order Placed')} className="btn-confirm">{t('cartmodalSubmitorder')}</button>
        </div>
      </div>
    </div>
  );
};
