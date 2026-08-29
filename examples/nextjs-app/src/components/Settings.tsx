import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';

export const SettingsPanel: React.FC = () => {
  const [emailNotifs, setEmailNotifs] = useState(true);

  return (
    <div className="settings-container">
      <h2>{t('accountSettings')}</h2>
      <div className="setting-item">
        <label>{t('emailNotifications')}</label>
        <input
          type="checkbox"
          checked={emailNotifs}
          onChange={(e) => setEmailNotifs(e.target.checked)}
        />
      </div>
      <div className="settings-btn-row">
        <button className="btn-save">{t('settingsSavechangesbtn')}</button>
        <button className="btn-delete">{t('settingsDeleteaccount')}</button>
      </div>
    </div>
  );
};
