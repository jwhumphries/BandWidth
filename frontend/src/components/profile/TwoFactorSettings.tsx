import {useState} from 'react';
import {useMe} from '../../hooks/auth';
import DisableTwoFactor from './DisableTwoFactor';
import EnrollTwoFactor from './EnrollTwoFactor';

export default function TwoFactorSettings() {
  const {data: user} = useMe();
  const [backupCodes, setBackupCodes] = useState<string[] | null>(null);

  if (!user) {
    return null;
  }
  if (backupCodes) {
    return (
      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Two-factor authentication enabled</h2>
          <p>
            Save these one-time backup codes somewhere safe — they will not be
            shown again.
          </p>
          <pre className="bg-base-200 rounded-box p-4">
            {backupCodes.join('\n')}
          </pre>
          <div className="card-actions justify-end">
            <button className="btn" onClick={() => setBackupCodes(null)}>
              I saved them
            </button>
          </div>
        </div>
      </section>
    );
  }
  return user.totpEnabled ? (
    <DisableTwoFactor />
  ) : (
    <EnrollTwoFactor onEnrolled={setBackupCodes} />
  );
}
