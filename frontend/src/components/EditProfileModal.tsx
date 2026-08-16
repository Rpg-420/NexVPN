import React, { useState, useEffect } from 'react'
import { X, Save, Edit3, Trash2, Server, Shield, Globe, Key, Tag, FileText, Check } from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, Protocol, ProfileCategory } from '../types'

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
  const [isSaving, setIsSaving] = useState(false)
  const [savedSuccess, setSavedSuccess] = useState(false)

  useEffect(() => {
    if (profile) {
      setName(profile.name || '')
      setServer(profile.server || '')
      setPort(profile.port || 443)
      setProtocol(profile.protocol || 'singbox')
      setCategory(profile.category || 'v2ray')
      setGroup(profile.group || '')
      setRawURI(profile.raw_uri || '')
      setConfigContent(profile.config_content || '')
      setUsername(profile.username || '')
      setPassword(profile.password || '')
      setWarpCleanIP(profile.warp_clean_ip || '')
      setSavedSuccess(false)
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
        updated_at: new Date().toISOString(),
      }

      await (api as any).updateProfile(updated)
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-2xl overflow-hidden shadow-2xl flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-900/80">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400">
              <Edit3 className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
                {t('edit_modal_title')}
                <span className="text-xs bg-slate-800 text-slate-300 px-2.5 py-0.5 rounded-full font-mono">
                  {profile.protocol.toUpperCase()}
                </span>
              </h3>
              <p className="text-xs text-slate-400">{t('edit_modal_subtitle')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-200 p-1.5 rounded-lg hover:bg-slate-800 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content Form */}
        <form onSubmit={handleSave} className="p-6 overflow-y-auto flex-1 space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Name */}
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
                <Tag className="w-3.5 h-3.5 text-cyan-400" />
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
              <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
                <Globe className="w-3.5 h-3.5 text-orange-400" />
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
              <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
                <Server className="w-3.5 h-3.5 text-emerald-400" />
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
              <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
                <Shield className="w-3.5 h-3.5 text-purple-400" />
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
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 bg-slate-800/40 border border-slate-700/50 p-3.5 rounded-xl">
              <div>
                <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
                  <Key className="w-3.5 h-3.5 text-amber-400" />
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
                <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
                  <Key className="w-3.5 h-3.5 text-amber-400" />
                  <span>{t('password_label')}</span>
                </label>
                <input
                  type="text"
                  className="form-control w-full text-xs font-mono"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  placeholder="vpn_pass"
                />
              </div>
            </div>
          )}

          {/* WARP Clean IP if applicable */}
          {(profile.protocol === 'warp' || profile.protocol === 'wireguard') && (
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
                <Globe className="w-3.5 h-3.5 text-orange-400" />
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

          {/* Raw URI */}
          {rawURI && (
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
                <FileText className="w-3.5 h-3.5 text-cyan-400" />
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
            <label className="text-xs font-semibold text-slate-300 block mb-1.5 flex items-center gap-1.5">
              <FileText className="w-3.5 h-3.5 text-slate-400" />
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
          <div className="flex items-center justify-between pt-4 border-t border-slate-800">
            <button
              type="button"
              onClick={handleDelete}
              className="flex items-center gap-1.5 text-rose-400 hover:text-rose-300 hover:bg-rose-500/10 px-3.5 py-2 rounded-xl text-xs font-semibold transition"
            >
              <Trash2 className="w-4 h-4" />
              <span>{t('delete_profile_btn')}</span>
            </button>

            <div className="flex items-center gap-2">
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={onClose}
              >
                {t('cancel')}
              </button>
              <button
                type="submit"
                disabled={isSaving}
                className="btn btn-primary btn-sm flex items-center gap-1.5"
              >
                {savedSuccess ? (
                  <>
                    <Check className="w-4 h-4 text-emerald-400" />
                    <span>{t('saved')}</span>
                  </>
                ) : (
                  <>
                    <Save className="w-4 h-4" />
                    <span>{isSaving ? t('saving') : t('save_changes_btn')}</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  )
}
