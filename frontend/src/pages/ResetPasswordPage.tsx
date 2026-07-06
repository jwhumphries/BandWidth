import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link, useSearchParams} from 'react-router';
import {useConfirmPasswordReset} from '../hooks/auth';

export default function ResetPasswordPage() {
  const [params] = useSearchParams();
  const token = params.get('token') ?? '';
  const confirm = useConfirmPasswordReset();
  const [newPassword, setNewPassword] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    confirm.mutate({token, newPassword});
  };

  return (
    <main className="hero bg-base-200 flex-1">
      <div className="hero-content w-full max-w-sm flex-col">
        <h1 className="text-4xl font-bold">Choose a new password</h1>
        {!token ? (
          <p>
            Invalid or expired reset link. Request a new one from the{' '}
            <Link className="link" to="/forgot-password">
              reset password page
            </Link>
            .
          </p>
        ) : confirm.isSuccess ? (
          <p>
            Password updated.{' '}
            <Link className="link" to="/login">
              Log in
            </Link>
          </p>
        ) : (
          <form
            className="card bg-base-100 w-full p-6 shadow"
            onSubmit={submit}
          >
            <fieldset className="fieldset">
              <label className="label" htmlFor="new-password">
                New password
              </label>
              <input
                id="new-password"
                type="password"
                className="input w-full"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                minLength={8}
                autoComplete="new-password"
                required
              />
              {confirm.error && (
                <div role="alert" className="alert alert-error mt-2">
                  {confirm.error.message}
                </div>
              )}
              <button
                className="btn btn-primary mt-4"
                disabled={confirm.isPending}
              >
                Set password
              </button>
            </fieldset>
          </form>
        )}
      </div>
    </main>
  );
}
