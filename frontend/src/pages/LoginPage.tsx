import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link, useNavigate} from 'react-router';
import {useAuthFeatures, useLogin} from '../hooks/auth';

export default function LoginPage() {
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [totpRequired, setTotpRequired] = useState(false);
  const navigate = useNavigate();
  const loginMutation = useLogin();
  const {data: features} = useAuthFeatures();

  const submit = (e: FormEvent) => {
    e.preventDefault();
    loginMutation.mutate(
      {login, password, ...(totpCode ? {totpCode} : {})},
      {
        onSuccess: () => void navigate('/'),
        onError: err => {
          if ((err.body as {totpRequired?: boolean} | null)?.totpRequired) {
            setTotpRequired(true);
          }
        },
      },
    );
  };

  const error =
    loginMutation.error &&
    !(loginMutation.error.body as {totpRequired?: boolean} | null)?.totpRequired
      ? loginMutation.error.message
      : null;

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col">
        <h1 className="text-4xl font-bold">BandWidth</h1>
        <form className="card bg-base-100 w-full p-6 shadow" onSubmit={submit}>
          <fieldset className="fieldset">
            <label className="label" htmlFor="login">
              Username or email
            </label>
            <input
              id="login"
              className="input w-full"
              value={login}
              onChange={e => setLogin(e.target.value)}
              required
            />
            <label className="label" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              className="input w-full"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
            />
            {totpRequired && (
              <>
                <label className="label" htmlFor="totp">
                  Two-factor code
                </label>
                <input
                  id="totp"
                  className="input w-full"
                  value={totpCode}
                  onChange={e => setTotpCode(e.target.value)}
                  placeholder="123456 or backup code"
                  autoComplete="one-time-code"
                  autoFocus
                />
              </>
            )}
            {error && (
              <div role="alert" className="alert alert-error mt-2">
                {error}
              </div>
            )}
            <button
              className="btn btn-primary mt-4"
              disabled={loginMutation.isPending}
            >
              Log in
            </button>
          </fieldset>
        </form>
        <p className="text-sm">
          No account?{' '}
          <Link className="link" to="/signup">
            Sign up
          </Link>
          {features?.passwordReset && (
            <>
              {' · '}
              <Link className="link" to="/forgot-password">
                Forgot password?
              </Link>
            </>
          )}
        </p>
      </div>
    </main>
  );
}
