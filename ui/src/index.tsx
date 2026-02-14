import { BookButton } from './BookButton';
import { StatusPanel } from './StatusPanel';

// ArgoCD UI extensions API — registered on the window object
((window: any) => {
  const extensionsAPI = window?.extensionsAPI;
  if (!extensionsAPI) {
    console.warn('[booking-ext] extensionsAPI not available');
    return;
  }

  // Register the "Booking" tab on Application resources
  extensionsAPI.registerResourceExtension(
    BookButton,
    'argoproj.io',
    'Application',
    'Booking',
    { icon: 'fa-lock' },
  );

  // Register the status panel badge
  extensionsAPI.registerStatusPanelExtension(
    StatusPanel,
    'Booking',
    'booking-status',
  );
})(window);
