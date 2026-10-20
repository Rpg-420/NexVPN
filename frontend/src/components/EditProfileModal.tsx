import React, { useState, useEffect } from 'react'
import { X, Save, Edit3, Trash2, Server, Shield, Globe, Key, Tag, FileText, Check, Eye, EyeOff, Gauge } from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, Protocol, ProfileCategory } from '../types'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  profile: Profile | null
  onClose: () => void
  onSaved: () => void
  onDeleted?: (id: string) => void
}

export default function EditProfileModal({ isOpen, profile, onClose, onSaved, onDeleted }: Props) {
  const [name, setName] = useState('')
  const [server, setServer] = useState('')
  const [port, setPort] = useState<number | string>(443)
  const [protocol, setProtocol] = useState<Protocol>('singbox')
  const [category, setCategory] = useState<ProfileCategory>('v2ray')
  const [group, setGroup] = useState('')
  const [rawURI, setRawURI] = useState('')
  const [configContent, setConfigContent] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [warpCleanIP, setWarpCleanIP] = useState('')
  // Kept as a string so the field can be empty; 0/blank means "no alert".
  const [dataCap, setDataCap] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [savedSuccess, setSavedSuccess] = useState(false)
  // The password field used to be type="text", so a saved credential was shown
  // in the clear on screen — in screenshots, screen shares and over the
  // shoulder. Masked by default, with an explicit reveal for the case where the
  // user needs to check what was typed.
  const [showPassword, setShowPassword] = useState(false)

  useEffect(() => {
    if (!profile) return
    let cancelled = false

    const applyProfile = (p: Profile) => {
      setName(p.name || '')
      setServer(p.server || '')
      setPort(p.port || 443)
      setProtocol(p.protocol || 'singbox')
      setCategory(p.category || 'v2ray')
      setGroup(p.group || '')
      setRawURI(p.raw_uri || '')
      setConfigContent(p.config_content || '')
      setUsername(p.username || '')
      setPassword(p.password || '')
      setWarpCleanIP(p.warp_clean_ip || '')
      setDataCap(p.session_data_cap_mb ? String(p.session_data_cap_mb) : '')
      setSavedSuccess(false)
    }

    // Render immediately from the row the user clicked, then re-read the
    // authoritative copy so an edit can never act on a stale list snapshot.
    applyProfile(profile)
    if (profile.id) {
      api
        .getProfileById(profile.id)
        .then(fresh => {
          if (!cancelled && fresh && fresh.id) applyProfile(fresh)
        })
        .catch(err => console.error('Failed to refresh profile:', err))
    }

    return () => {
      cancelled = true
    }
  }, [profile])

  if (!isOpen || !profile) return null

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    setIsSaving(true)
    try {
      const updated: Profile = {
        ...profile,
        name: name.trim(),
        server: server.trim(),
        port: typeof port === 'string' ? parseInt(port) || 443 : port,
        protocol: protocol,
        category: category,
        group: group.trim() || 'Custom',
        raw_uri: rawURI.trim(),
        config_content: configContent.trim(),
        username: username.trim(),
        password: password.trim(),
        warp_clean_ip: warpCleanIP.trim(),
        // Blank clears the alert; a non-numeric entry is treated as "no alert"
        // rather than rejected, because the field is advisory.
        session_data_cap_mb: Math.max(0, Math.floor(Number(dataCap) || 0)),
        updated_at: new Date().toISOString(),
      }

      await api.updateProfile(updated)
      setSavedSuccess(true)
      setTimeout(() => {
        onSaved()
        onClose()
      }, 500)
    } catch (err) {
      console.error('Update profile error:', err)
      alert(t('error'))
    } finally {
      setIsSaving(false)
    }
  }

  const handleDelete = async () => {
    if (confirm(t('confirm_delete'))) {
      try {
        await api.deleteProfile(profile.id)
        if (onDeleted) onDeleted(profile.id)
        onClose()
      } catch (err) {
        console.error('Delete error:', err)
      }
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      className="max-h-[90vh]"
    >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border bg-card">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl border border-border bg-muted flex items-center justify-center text-muted-foreground">
              <Edit3 className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-foreground flex items-center gap-2">
                {t('edit_modal_title')}
                <span className="text-xs bg-muted text-muted-foreground px-2.5 py-0.5 rounded-full font-mono">
                  {profile.protocol.toUpperCase()}
                </span>
              </h3>
              <p className="text-xs text-muted-foreground">{t('edit_modal_subtitle')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground p-1.5 rounded-lg hover:bg-muted transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content Form */}
        <form onSubmit={handleSave} className="p-6 overflow-y-auto flex-1 space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Name */}
            <div>
              <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                <Tag className="w-3.5 h-3.5 text-primary" />
                <span>{t('profile_name_label')}</span>
              </label>
              <input
                type="text"
                className="form-control w-full text-xs font-medium"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder={t('profile_name_placeholder')}
                required
              />
            </div>

            {/* Group */}
            <div>
              <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                <Globe className="w-3.5 h-3.5 text-warning" />
                <span>{t('group_category_label')}</span>
              </label>
              <input
                type="text"
                className="form-control w-full text-xs"
                value={group}
                onChange={e => setGroup(e.target.value)}
                placeholder={t('group_placeholder')}
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Server IP/Host */}
            <div className="md:col-span-2">
              <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                <Server className="w-3.5 h-3.5 text-success" />
                <span>{t('server_host_label')}</span>
              </label>
              <input
                type="text"
                className="form-control w-full font-mono text-xs"
                value={server}
                onChange={e => setServer(e.target.value)}
                placeholder={t('server_placeholder')}
              />
            </div>

            {/* Port */}
            <div>
              <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                <Shield className="w-3.5 h-3.5 text-primary" />
                <span>{t('port_label_edit')}</span>
              </label>
              <input
                type="number"
                className="form-control w-full font-mono text-xs"
                value={port}
                onChange={e => setPort(e.target.value)}
                placeholder="443"
              />
            </div>
          </div>

          {/* OpenVPN Credentials if applicable */}
          {(profile.protocol === 'openvpn' || profile.category === 'openvpn') && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 bg-muted border border-border p-3.5 rounded-xl">
              <div>
                <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                  <Key className="w-3.5 h-3.5 text-warning" />
                  <span>{t('username_label')}</span>
                </label>
                <input
                  type="text"
                  className="form-control w-full text-xs font-mono"
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                  placeholder="vpn_user"
                />
              </div>

              <div>
                <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                  <Key className="w-3.5 h-3.5 text-warning" />
                  <span>{t('password_label')}</span>
                </label>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="off"
                    className="form-control w-full text-xs font-mono pr-9"
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    placeholder="vpn_pass"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(v => !v)}
                    aria-label={showPassword ? t('hide_password') : t('show_password')}
                    title={showPassword ? t('hide_password') : t('show_password')}
                    className="absolute inset-y-0 right-0 flex items-center px-2.5 text-muted-foreground hover:text-foreground transition"
                  >
                    {showPassword ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* WARP Clean IP if applicable */}
          {(profile.protocol === 'warp' || profile.protocol === 'wireguard') && (
            <div>
              <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                <Globe className="w-3.5 h-3.5 text-warning" />
                <span>{t('warp_clean_ip_modal_label')}</span>
              </label>
              <input
                type="text"
                className="form-control w-full font-mono text-xs"
                value={warpCleanIP}
                onChange={e => setWarpCleanIP(e.target.value)}
                placeholder="162.159.192.1"
              />
            </div>
          )}

          {/* Session data alert */}
          <div>
            <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
              <Gauge className="w-3.5 h-3.5 text-muted-foreground" />
              <span>{t('adv_data_cap')}</span>
            </label>
            <input
              type="number"
              min={0}
              step={100}
              className="form-control w-full text-xs font-mono"
              value={dataCap}
              onChange={e => setDataCap(e.target.value)}
              placeholder={t('adv_data_cap_placeholder')}
            />
            <p className="text-[10px] text-muted-foreground mt-1 leading-relaxed">
              {t('adv_data_cap_hint')}
            </p>
          </div>

          {/* Raw URI */}
          {rawURI && (
            <div>
              <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                <FileText className="w-3.5 h-3.5 text-primary" />
                <span>{t('raw_uri_label')}</span>
              </label>
              <textarea
                rows={3}
                className="form-control w-full font-mono text-xs"
                value={rawURI}
                onChange={e => setRawURI(e.target.value)}
                placeholder="vless://..."
              />
            </div>
          )}

          {/* Config Content */}
          <div>
            <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
              <FileText className="w-3.5 h-3.5 text-muted-foreground" />
              <span>{t('config_content_label')}</span>
            </label>
            <textarea
              rows={6}
              className="form-control w-full font-mono text-xs"
              value={configContent}
              onChange={e => setConfigContent(e.target.value)}
              placeholder={t('config_content_placeholder')}
            />
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-between pt-4 border-t border-border">
            <button
              type="button"
              onClick={handleDelete}
              className="flex items-center gap-1.5 text-destructive hover:text-destructive hover:bg-destructive/10 px-3.5 py-2 rounded-xl text-xs font-semibold transition"
            >
              <Trash2 className="w-4 h-4" />
              <span>{t('delete_profile_btn')}</span>
            </button>

            <div className="flex items-center gap-2">
              <Button variant="secondary" size="sm"
                type="button"
                onClick={onClose}
              >
                {t('cancel')}
              </Button>
              <Button size="sm" className="flex items-center gap-1.5"
                type="submit"
                disabled={isSaving}
              >
                {savedSuccess ? (
                  <>
                    <Check className="w-4 h-4 text-success" />
                    <span>{t('saved')}</span>
                  </>
                ) : (
                  <>
                    <Save className="w-4 h-4" />
                    <span>{isSaving ? t('saving') : t('save_changes_btn')}</span>
                  </>
                )}
              </Button>
            </div>
          </div>
        </form>
    </ModalShell>
  )
}
