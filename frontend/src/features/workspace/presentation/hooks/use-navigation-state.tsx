import React from 'react'

export function useNavigationState() {
  const [navState, setNavState] = React.useState({
    canGoBack: false,
    canGoForward: false,
  })

  React.useEffect(() => {
    if (typeof window === 'undefined') return;

    if (!('navigation' in window)) {
      setNavState({
        canGoBack: (window as any).history.length > 1,
        canGoForward: false,
      })
      return;
    }

    const updateNav = () => {
      setNavState({
        canGoBack: (window as any).navigation.canGoBack,
        canGoForward: (window as any).navigation.canGoForward,
      })
    }

    updateNav()

    const nav = (window as any).navigation
    nav.addEventListener('currententrychange', updateNav)

    return () => {
      nav.removeEventListener('currententrychange', updateNav)
    }
  }, [])

  return navState
}