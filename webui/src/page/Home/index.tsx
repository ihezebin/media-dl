import {
  ArrowRightOutlined,
  CloudDownloadOutlined,
  GithubOutlined,
  SoundOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'

import styles from './index.module.scss'

const WORKFLOWS = [
  {
    key: 'video',
    index: '01',
    eyebrow: 'VIDEO / DOWNLOAD',
    title: '视频下载',
    description: '粘贴链接，快速解析并保存高清视频。',
    detail: 'Bilibili · 抖音 · 小红书 · 更多平台',
    icon: <VideoCameraOutlined />,
  },
  {
    key: 'music',
    index: '02',
    eyebrow: 'MUSIC / SEARCH',
    title: '音乐搜索',
    description: '跨平台搜索歌曲，下载音频、歌词和封面。',
    detail: '网易云 · QQ · 酷狗 · Apple Music',
    icon: <SoundOutlined />,
  },
] as const

export default function Home() {
  const navigate = useNavigate()

  return (
    <div className={styles.page}>
      <div className={styles.backdrop} aria-hidden="true">
        <div className={styles.backdropGrid} />
        <div className={styles.backdropLine} />
        <div className={styles.backdropMark}>MD / 01</div>
      </div>

      <main className={styles.homeContent}>
        <section className={styles.hero}>
          <div className={styles.eyebrow}>
            <span className={styles.eyebrowDot} />
            OPEN-SOURCE MEDIA TOOLKIT
            <span className={styles.eyebrowLine} />
          </div>
          <h1>
            让喜欢的媒体
            <span>自由流动。</span>
          </h1>
          <p className={styles.heroCopy}>
            从链接到本地，一套工具完成视频下载与音乐搜索。
            <br />
            简单、快速，保持你想要的清晰度与完整信息。
          </p>
          <div className={styles.heroSignal}>
            <CloudDownloadOutlined />
            <span>选择一个工作流开始</span>
            <i />
            <strong>POWERED BY MUSIC-LIB</strong>
          </div>
        </section>

        <section className={styles.mediaSummary} aria-label="媒体下载能力说明">
          <div className={styles.summaryLead}>
            <span className={styles.summaryEyebrow}>MEDIA / 01</span>
            <h2>一个入口，处理两类媒体。</h2>
            <p>视频按链接提取，音乐按关键词搜索；需要的文件、封面与歌词，都在同一个工作区完成。</p>
          </div>
          <div className={styles.summaryFacts}>
            <div><strong>VIDEO</strong><span>链接解析 · 高清下载</span></div>
            <div><strong>MUSIC</strong><span>跨平台搜索 · 音频保存</span></div>
            <div><strong>ASSETS</strong><span>封面与歌词 · 一并获取</span></div>
          </div>
        </section>

        <section className={styles.workflowGrid} aria-label="选择下载功能">
          {WORKFLOWS.map((workflow) => (
            <button
              key={workflow.key}
              type="button"
              className={`${styles.workflow} ${workflow.key === 'video' ? styles.workflowVideo : styles.workflowMusic}`}
              onClick={() => navigate(`/${workflow.key}`)}>
              <span className={styles.workflowGlow} aria-hidden="true" />
              <span className={styles.workflowTop}>
                <span className={styles.workflowIndex}>{workflow.index}</span>
                <span className={styles.workflowEyebrow}>{workflow.eyebrow}</span>
                <span className={styles.workflowArrow}><ArrowRightOutlined /></span>
              </span>
              <span className={styles.workflowBody}>
                <span className={styles.workflowIcon}>{workflow.icon}</span>
                <span className={styles.workflowText}>
                  <strong>{workflow.title}</strong>
                  <span>{workflow.description}</span>
                  <small>{workflow.detail}</small>
                </span>
              </span>
              <span className={styles.workflowViz} aria-hidden="true">
                {workflow.key === 'video' ? (
                  <>
                    <i className={styles.videoScreen} />
                    <i className={styles.videoPlay} />
                    <i className={styles.videoBarOne} />
                    <i className={styles.videoBarTwo} />
                    <i className={styles.videoBarThree} />
                  </>
                ) : (
                  <>
                    <i className={styles.musicDisc} />
                    <i className={styles.musicArm} />
                    <i className={styles.musicWaveOne} />
                    <i className={styles.musicWaveTwo} />
                    <i className={styles.musicWaveThree} />
                  </>
                )}
              </span>
            </button>
          ))}
        </section>
        <footer className={styles.footerRail}>
          <span>MEDIA-DL / MULTI-PLATFORM DOWNLOAD</span>
          <a href="https://github.com/ihezebin/media-dl" target="_blank" rel="noreferrer">
            <GithubOutlined />
            VIEW ON GITHUB
          </a>
        </footer>
      </main>
    </div>
  )
}
