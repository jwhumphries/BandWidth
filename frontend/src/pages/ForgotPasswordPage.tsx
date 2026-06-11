import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link} from 'react-router';
import {useRequestPasswordReset} from '../hooks/auth';

export default function ForgotPasswordPage() {
  const request = useRequestPasswordReset();
  const [email, setEmail] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    request.mutate({email});
  };

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col">
        <h1 className="text-4xl font-bold">Reset password</h1>
        {request.isSuccess ? (
          <p>
            If an account exists for that address, a reset link is on its way.
            Check your inbox.
          </p>
        ) : (
          <form
            className="card bg-base-100 w-full p-6 shadow"
            onSubmit={submit}
          >
            <fieldset className="fieldset">
              <label className="label" htmlFor="email">
                Email
              </label>
              <input
                id="email"
                type="email"
                className="input w-full"
                value={email}
                onChange={e => setEmail(e.target.value)}
                required
              />
              {request.error && (
                <div role="alert" className="alert alert-error mt-2">
                  {request.error.message}
                </div>
              )}
              <button
                className="btn btn-primary mt-4"
                disabled={request.isPending}
              >
                Send reset link
              </button>
            </fieldset>
          </form>
        )}
        <p className="text-sm">
          <Link className="link" to="/login">
            Back to login
          </Link>
        </p>
      </div>
    </main>
  );
}
