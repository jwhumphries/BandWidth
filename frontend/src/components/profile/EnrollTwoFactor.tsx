import {useEffect, useState} from 'react';
import type {FormEvent} from 'react';
import QRCode from 'qrcode';
import {useTwoFactorSetup, useTwoFactorVerify} from '../../hooks/auth';

export default function EnrollTwoFactor({
  onEnrolled,
}: {
  onEnrolled: (codes: string[]) => void;
}) {
  const setup = useTwoFactorSetup();
  const verify = useTwoFactorVerify();
  const [code, setCode] = useState('');
  const [qr, setQr] = useState<string | null>(null);

  useEffect(() => {
    if (setup.data) {
      QRCode.toDataURL(setup.data.otpauthUrl)
        .then(setQr)
        .catch(() => setQr(null));
    }
  }, [setup.data]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    verify.mutate({code}, {onSuccess: data => onEnrolled(data.backupCodes)});
  };

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Two-factor authentication</h2>
        {setup.data ? (
          <form onSubmit={submit}>
            <p>
              Scan the QR code with your authenticator app, then enter a code to
              confirm.
            </p>
            {qr && (
              <img
                src={qr}
                alt="TOTP enrollment QR code"
                className="mx-auto my-4"
              />
            )}
            <p className="text-sm">
              Manual entry secret:{' '}
              <span className="font-mono">{setup.data.secret}</span>
            </p>
            <label className="label" htmlFor="verify-code">
              Code
            </label>
            <input
              id="verify-code"
              className="input w-full"
              value={code}
              onChange={e => setCode(e.target.value)}
              autoComplete="one-time-code"
              required
            />
            {verify.error && (
              <div role="alert" className="alert alert-error mt-2">
                {verify.error.message}
              </div>
            )}
            <div className="card-actions justify-end mt-4">
              <button className="btn btn-primary" disabled={verify.isPending}>
                Confirm
              </button>
            </div>
          </form>
        ) : (
          <>
            <p>Protect your account with an authenticator app.</p>
            {setup.error && (
              <div role="alert" className="alert alert-error">
                {setup.error.message}
              </div>
            )}
            <div className="card-actions justify-end">
              <button
                className="btn btn-primary"
                onClick={() => setup.mutate()}
                disabled={setup.isPending}
              >
                Enable 2FA
              </button>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
