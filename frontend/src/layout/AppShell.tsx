import { Outlet } from 'react-router'

import { AnalysisProvider } from '../features/analysis/AnalysisProvider'
import { AnalysisBanner } from './AnalysisBanner'
import { Header } from './Header'
import { Sidebar } from './Sidebar'

/** Signed-in layout: sidebar, header with Run AI Analysis, run banner and the page. */
export function AppShell() {
  return (
    <AnalysisProvider>
      <div className="flex h-full">
        <Sidebar />
        <div className="flex min-w-0 flex-1 flex-col">
          <Header />
          <AnalysisBanner />
          <main className="flex-1 overflow-y-auto px-8 py-6">
            <Outlet />
          </main>
        </div>
      </div>
    </AnalysisProvider>
  )
}
