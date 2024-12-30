document.addEventListener('DOMContentLoaded', () => {
  const apiUrl = 'http://localhost:8081/links';
  const linksList = document.getElementById('links-list');
  const modal = document.getElementById('modal');
  const modalBody = document.getElementById('modal-body');
  const closeModal = document.getElementById('close-modal');
  const darkModeToggle = document.getElementById('dark-mode-toggle');
  const nsfwToggle = document.getElementById('nsfw-toggle');

  // Initial Settings
  const settings = {
    darkMode: localStorage.getItem('darkMode') === 'false' ? false : true,
    hideNSFW: localStorage.getItem('hideNSFW') === 'false' ? false : true,
  };

  document.body.dataset.theme = settings.darkMode ? 'dark' : 'light';
  darkModeToggle.checked = settings.darkMode;
  nsfwToggle.checked = settings.hideNSFW;

  // Fetch Data from API
  async function fetchLinks() {
    try {
      const response = await fetch(apiUrl);
      const data = await response.json();
      renderLinks(data);
    } catch (error) {
      console.error('Error fetching data:', error);
    }
  }

  // Render Links
  function renderLinks(data) {
    linksList.innerHTML = '';
    data.forEach((entry) => {
      const { date, links } = entry;

      // Date Header
      const dateHeader = document.createElement('li');
      dateHeader.className = 'date-header';
      dateHeader.textContent = date;
      linksList.appendChild(dateHeader);

      // Links
      links.forEach((link) => {
        const isNSFW = link.nsfw;
        if (settings.hideNSFW && isNSFW) return;

        const listItem = document.createElement('li');
        listItem.className = isNSFW ? 'nsfw' : '';
        listItem.innerHTML = `
          <a href="#" data-type="${link.type}" data-src="${link.src}" class="link">
            <img src="icons/${link.icon.toLowerCase()}.png" alt="${link.icon} icon">
            ${link.title}
            ${isNSFW ? '<span class="nsfw-icon">(NSFW)</span>' : ''}
          </a>
          ${
            link.comment_url
              ? `<a href="#" class="comment-link" data-comment-url="${link.comment_url}">
                  <div class="comment-icon">
                    <span class="comment-number">${link.comment_number}</span>
                  </div>
                </a>`
              : ''
          }
        `;
        linksList.appendChild(listItem);
      });
    });

    // Add event listeners to links
    document.querySelectorAll('.link').forEach((link) => {
      link.addEventListener('click', handleLinkClick);
    });

    // Add event listeners to comment links
    document.querySelectorAll('.comment-link').forEach((commentLink) => {
      commentLink.addEventListener('click', handleCommentLinkClick);
    });
  }

  // Handle Link Click
  function handleLinkClick(event) {
    event.preventDefault();
    const { type, src } = event.target.closest('a').dataset;

    if (type === 'redirect' || type === 'iframe') {
      window.open(src, '_blank');
    } else if (type === 'youtube') {
      showModal(`
        <iframe src="https://www.youtube.com/embed/${src}" frameborder="0" allowfullscreen></iframe>
      `);
    } else if (src.endsWith('.png') || src.endsWith('.jpg') || src.endsWith('.jpeg') || src.endsWith('.gif')) {
      showModal(`<img src="${src}" alt="Image">`);
    } else if (src.endsWith('.mp4')) {
      showModal(`
        <video controls>
          <source src="${src}" type="video/mp4">
          Your browser does not support the video tag.
        </video>
      `);
    }
  }

  // Handle Comment Link Click
  function handleCommentLinkClick(event) {
    event.preventDefault();
    const commentUrl = event.target.closest('a').dataset.commentUrl;
    showModal(`
      <iframe src="https://existenz.se/${commentUrl}" frameborder="0" width="100%" height="100%"></iframe>
    `);
  }

  // Show Modal
  function showModal(content) {
    modalBody.innerHTML = content;
    modal.classList.add('open');
  }

  // Close Modal
  closeModal.addEventListener('click', closeModalHandler);
  modal.addEventListener('click', (e) => {
    if (e.target === modal) closeModalHandler();
  });

  function closeModalHandler() {
    modal.classList.remove('open');
    modalBody.innerHTML = '';
  }

  // Dark Mode Toggle
  darkModeToggle.addEventListener('change', (event) => {
    document.body.dataset.theme = event.target.checked ? 'dark' : 'light';
    localStorage.setItem('darkMode', event.target.checked);
  });

  // NSFW Toggle
  nsfwToggle.addEventListener('change', (event) => {
    settings.hideNSFW = event.target.checked;
    localStorage.setItem('hideNSFW', event.target.checked);
    // Rerender links with updated visibility
    fetchLinks();
  });

  // Initial Fetch
  fetchLinks();
});
