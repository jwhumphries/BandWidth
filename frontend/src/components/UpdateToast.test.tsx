import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {render} from '@testing-library/react';

const updateServiceWorker = vi.fn();
let needRefresh = true;

vi.mock('virtual:pwa-register/react', () => ({
  useRegisterSW: () => ({
    needRefresh: [needRefresh, vi.fn()],
    offlineReady: [false, vi.fn()],
    updateServiceWorker,
  }),
}));

// Import AFTER the mock is registered.
import UpdateToast from './UpdateToast';

describe('UpdateToast', () => {
  beforeEach(() => {
    updateServiceWorker.mockClear();
    needRefresh = true;
  });

  it('shows a reload prompt when an update is ready and reloads on click', async () => {
    render(<UpdateToast />);
    expect(screen.getByText(/new version available/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: /reload/i}));
    expect(updateServiceWorker).toHaveBeenCalledWith(true);
  });

  it('renders nothing when no update is pending', () => {
    needRefresh = false;
    const {container} = render(<UpdateToast />);
    expect(container).toBeEmptyDOMElement();
  });
});
