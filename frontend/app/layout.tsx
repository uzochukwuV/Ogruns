import type { Metadata, Viewport } from 'next'
import { JetBrains_Mono } from 'next/font/google'
import { GeistPixelGrid } from 'geist/font/pixel'
import { ThemeProvider } from '@/components/theme-provider'

import './globals.css'

const jetbrainsMono = JetBrains_Mono({
  subsets: ['latin'],
  variable: '--font-mono',
})

export const metadata: Metadata = {
  title: '0G Signal Marketplace | Decentralized AI Trading Signals',
  description:
    'A decentralized marketplace where AI trading agents publish encrypted market signals, verified and scored based on real market outcomes. Subscribe to proven signal providers with transparent trust scores.',
  keywords: [
    '0G',
    'signal marketplace',
    'AI trading signals',
    'decentralized trading',
    'crypto signals',
    'trust score',
    'verified signals',
    'AI agents',
    'trading bot',
    'market signals',
    'encrypted signals',
    'DeFi',
    'Web3',
  ],
  authors: [{ name: '0G Labs' }],
  creator: '0G Labs',
  publisher: '0G Labs',
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      'max-video-preview': -1,
      'max-image-preview': 'large',
      'max-snippet': -1,
    },
  },
  openGraph: {
    type: 'website',
    locale: 'en_US',
    title: '0G Signal Marketplace | Decentralized AI Trading Signals',
    description:
      'Subscribe to verified AI trading signals with transparent trust scores. Decentralized verification ensures no fake track records.',
    siteName: '0G Signal Marketplace',
  },
  twitter: {
    card: 'summary_large_image',
    title: '0G Signal Marketplace',
    description:
      'Decentralized marketplace for verified AI trading signals. Transparent trust scores. Real market verification.',
    creator: '@0aboratory',
  },
  category: 'finance',
}

export const viewport: Viewport = {
  themeColor: '#F2F1EA',
  width: 'device-width',
  initialScale: 1,
  maximumScale: 5,
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en" className={`${jetbrainsMono.variable} ${GeistPixelGrid.variable}`} suppressHydrationWarning>
      <body className="font-mono antialiased">
        <ThemeProvider attribute="class" defaultTheme="light" enableSystem={false} disableTransitionOnChange>
          {children}
        </ThemeProvider>
      </body>
    </html>
  )
}
