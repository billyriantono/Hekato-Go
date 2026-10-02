// The authenticated console: router, shell and pages. Loaded as its own chunk so
// the login screen ships without any of it.
import { RouterProvider } from '@tanstack/react-router'
import { useEffect } from 'react'
import { prefetchPages, router } from '@/router'

export default function App() {
  useEffect(prefetchPages, [])
  return <RouterProvider router={router} />
}
