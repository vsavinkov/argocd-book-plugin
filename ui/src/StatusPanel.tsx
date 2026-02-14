import * as React from 'react';
import { BookingStatus, getStatus, bookApp, unbookApp } from './api';

const { useState, useEffect, useRef } = React;

const BUTTON_ID = 'booking-toolbar-btn';

export const StatusPanel: React.FC<any> = ({ application }) => {
  const [status, setStatus] = useState<BookingStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const stateRef = useRef({ status, loading, error });
  stateRef.current = { status, loading, error };

  const appName = application?.metadata?.name;
  const appNamespace = application?.metadata?.namespace;
  const project = application?.spec?.project;
  const appIdentifier = appNamespace && appName ? `${appNamespace}:${appName}` : '';

  const fetchStatusRef = useRef<() => Promise<void>>();
  fetchStatusRef.current = async () => {
    if (!appIdentifier || !project) return;
    try {
      const s = await getStatus(appIdentifier, project);
      setStatus(s);
      setError(null);
    } catch (e: any) {
      setError(e.message);
    }
  };

  useEffect(() => {
    if (appIdentifier && project) fetchStatusRef.current?.();
  }, [appName, appNamespace]);

  // Inject button into toolbar
  useEffect(() => {
    if (!appIdentifier) return;

    const handleClick = async () => {
      const { status, loading } = stateRef.current;
      if (loading) return;
      setLoading(true);
      try {
        if (status?.booked) {
          await unbookApp(appIdentifier, project);
        } else {
          await bookApp(appIdentifier, project);
        }
        await fetchStatusRef.current?.();
      } catch (e: any) {
        setError(e.message);
      } finally {
        setLoading(false);
      }
    };

    const inject = () => {
      if (document.getElementById(BUTTON_ID)) return;
      const allButtons = Array.from(document.querySelectorAll('button'));
      const refreshBtn = allButtons.find(
        (b) => (b.textContent || '').toUpperCase().includes('REFRESH')
      );
      if (!refreshBtn?.parentNode) return;

      const btn = document.createElement('button');
      btn.id = BUTTON_ID;
      btn.className = 'argo-button argo-button--base';
      btn.style.cssText = 'display:inline-flex;align-items:center;gap:5px;margin-right:2px;';
      btn.addEventListener('click', handleClick);
      refreshBtn.parentNode.insertBefore(btn, refreshBtn);
    };

    inject();
    const observer = new MutationObserver(() => {
      if (!document.getElementById(BUTTON_ID)) inject();
    });
    observer.observe(document.body, { childList: true, subtree: true });

    return () => {
      observer.disconnect();
      const el = document.getElementById(BUTTON_ID);
      if (el) el.remove();
    };
  }, [appIdentifier, project]);

  // Update button appearance on state change
  useEffect(() => {
    const btn = document.getElementById(BUTTON_ID) as HTMLButtonElement | null;
    if (!btn) return;

    btn.disabled = false;
    btn.style.backgroundColor = '';
    btn.style.borderColor = '';
    btn.title = '';

    if (error) {
      btn.innerHTML = '<i class="fa fa-exclamation-triangle"></i> BOOK';
      btn.title = error;
    } else if (!status) {
      btn.innerHTML = '<i class="fa fa-spinner fa-spin"></i>';
      btn.disabled = true;
    } else if (loading) {
      btn.innerHTML = '<i class="fa fa-spinner fa-spin"></i> ' +
        (status.booked ? 'UNBOOKING...' : 'BOOKING...');
      btn.disabled = true;
    } else if (status.booked) {
      btn.innerHTML = '<i class="fa fa-lock"></i> BOOKED: ' + status.bookedBy;
      btn.style.backgroundColor = '#e96d76';
      btn.style.borderColor = '#e96d76';
      btn.title = 'Booked by ' + status.bookedBy + ' \u2014 click to unbook';
    } else {
      btn.innerHTML = '<i class="fa fa-unlock"></i> BOOK';
      btn.title = 'Book this application for exclusive use';
    }
  }, [status, loading, error]);

  // Render nothing in the status panel slot
  return null;
};
