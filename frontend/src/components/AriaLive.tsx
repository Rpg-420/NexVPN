import { useEffect, useState } from 'react'
import { setAnnouncer } from '../lib/announcer'

/**
 * Mounted once at the app root. Renders two visually-hidden ARIA live regions
 * (one polite, one assertive) and registers the announcer bridge so any part of
 * the app can surface messages to assistive technology via announce().
 */
export default function AriaLive() {
  const [polite, setPolite] = useState('')
  const [assertive, setAssertive] = useState('')

  useEffect(() => {
    setAnnouncer((message, politeness) => {
      const set = politeness === 'assertive' ? setAssertive : setPolite
      // Clear first so an identical consecutive message is still announced.
      set('')
      window.setTimeout(() => set(message), 30)
    })
    return () => setAnnouncer(null)
  }, [])

  return (
    <>
      <div aria-live="polite" aria-atomic="true" className="sr-only">
        {polite}
      </div>
      <div aria-live="assertive" aria-atomic="true" className="sr-only">
        {assertive}
      </div>
    </>
  )
}
