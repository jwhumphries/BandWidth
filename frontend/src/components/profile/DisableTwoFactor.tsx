import {useState} from 'react';
import type {FormEvent} from 'react';
import {useTwoFactorDisable} from '../../hooks/auth';

export default function DisableTwoFactor() {
  const disable = useTwoFactorDisable();
  const [code, setCode] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    disable.mutate({code});
  };

  return (
    <section className="card bg-base-100 shadow">
      <form className="card-body" onSubmit={submit}>
        <h2 className="card-title">Two-factor authentication</h2>
        <p>2FA is enabled on your account.</p>
        <label className="label" htmlFor="disable-code">
          Enter a current code (or backup code) to disable
        </label>
        <input
          id="disable-code"
          className="input w-full"
          value={code}
          onChange={e => setCode(e.target.value)}
          autoComplete="one-time-code"
          required
        />
        {disable.error && (
          <div role="alert" className="alert alert-error">
            {disable.error.message}
          </div>
        )}
        <div className="card-actions justify-end">
          <button className="btn btn-warning" disabled={disable.isPending}>
            Disable 2FA
          </button>
        </div>
      </form>
    </section>
  );
}
