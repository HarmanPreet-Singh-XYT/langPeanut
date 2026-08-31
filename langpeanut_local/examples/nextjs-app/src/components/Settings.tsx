import React, { useState } from 'react';

export const SettingsPanel: React.FC = () => {
  const [emailNotifs, setEmailNotifs] = useState(true);

  return (
    <div className="settings-container">
      <h2>Account Settings</h2>
      <div className="setting-item">
        <label>Email Notifications</label>
        <input
          type="checkbox"
          checked={emailNotifs}
          onChange={(e) => setEmailNotifs(e.target.checked)}
        />
      </div>
      <div className="settings-btn-row">
        <button className="btn-save">Save</button>
        <button className="btn-delete">Delete Account</button>
      </div>
    </div>
  );
};
