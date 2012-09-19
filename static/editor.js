$(function() {
	(function ($) {

		function Editor($el, settings) {
			this.$el = $el;
			$.extend(this, settings);

			var editor = this;

			this.setUp();
		}

		Editor.prototype.start = function () {
			this.$el.find('.post').show();

			var $body = this.$el.find('.body');
			$body.focus();
			window.getSelection().selectAllChildren($body.get(0));

			$(document).unbind('keypress.p');
			return false;
		};

		Editor.prototype.submit = function () {
			var editor = this;
			var $body = editor.$el.find('.body');

			data = {
				'html': $.trim($body.html())
			};
			if (!data['html'])
				return false;

			$.ajax({
				url: '/post',
				type: 'POST',
				dataType: 'json',
				data: data,
				success: function (data, textStatus, xhr) {
					editor.reset();

					// Add the new one.
					var $oldpost = editor.$el.find('.post');
					var $post = $oldpost.clone();
					$post.find('.body').html(data['Html']);
					var $permalink = $post.find('.time a');
					var posted = new Date(Date.parse(data['Posted']));
					$permalink.attr('href', '/' + posted.getUTCFullYear() + '/' + data['Id']);
					var postedText = $.relatizeDate.strftime(posted, "%i:%M <small>%p</small> %D %b %Y");
					$permalink.html(postedText);
					$oldpost.after($post);
					$post.show();
				},
				error: function (xhr, textStatus, errorThrown) {
					alert('ERROR: ' + xhr.responseText);
				}
			});
			return false;
		};

		Editor.prototype.reset = function () {
			var $body = this.$el.find('.body');
			$body.blur();
			$body.text('new post');

			this.$el.find('.post').hide();

			$(document).bind('keypress.p', this.start.bind(this));
			return false;
		};

		Editor.prototype.setUp = function () {
			var $body = this.$el.find('.body');
			$body.bind('keydown.return', this.submit.bind(this));
			$body.bind('keydown.esc', this.reset.bind(this));
			$body.bind('keydown.ctrl_l', this.makeLink.bind(this));

			$(document).bind('keypress.p', this.start.bind(this));

			var editor = this;
			var $linkEditor = this.$el.find('.link-editor');
			$linkEditor.hide();
			$linkEditor.blur(function (e) { return editor.deactivateLinkEditor() });
			$body.find('a').live('click', function (e) { editor.activateLinkEditor($(e.target)); return false })
				.live('mouseover', function (e) { return editor.showLinkEditor($(e.target)) })
				.live('mouseout', function (e) { return editor.deactivateLinkEditor() });
		};

		Editor.prototype.makeLink = function () {
			// TODO: if no text is selected, insert and select some ("link"?)
			// TODO: if the cursor is inside a link, just edit the existing link
			document.execCommand('createLink', false, ' ');
			var $link = $(window.getSelection().anchorNode).parent();
			$link.attr('href', '');
			this.activateLinkEditor($link);
		};

		Editor.prototype.showLinkEditor = function ($link) {
			if ($link.parents('[contenteditable="false"]').size())
				return;
			var linkpos = $link.offset();

			var $linkeditor = this.$el.find('.link-editor');
			$linkeditor.text($link.attr('href'));
			$linkeditor.bind('keyup', function (e) {
				$link.attr('href', $(this).text());
				$linkeditor.css('backgroundColor', $link.css('color'));
			});
			$linkeditor.bind('keydown', function (e) {
				if (e.altKey || e.shiftKey || e.ctrlKey)
					return true;
				if (e.which != 9 && e.which != 13)
					return true;
				$link.focus();
				window.getSelection().collapse($link.get(0), 1);
				return false;
			});
			$linkeditor.show();
			$linkeditor.offset({ top: linkpos.top + $link.height(), left: linkpos.left });
		};

		Editor.prototype.activateLinkEditor = function ($link) {
			if ($link.parents('[contenteditable="false"]').size())
				return;
			this.showLinkEditor($link);
			var $linkeditor = this.$el.find('.link-editor');
			$linkeditor.focus();
			window.getSelection().selectAllChildren($linkeditor.get(0));
		};

		Editor.prototype.deactivateLinkEditor = function () {
			var $linkeditor = this.$el.find('.link-editor');
			$linkeditor.unbind('keyup');
			$linkeditor.unbind('keydown');
			$linkeditor.hide();
		};

		$.fn.editor = function (options) {
			var settings = {
				autosaveid: '',
				ondiscard: function () {},
				lastSetting: null
			};
			$.extend(settings, options);

			return this.each(function () {
				var $this = $(this);
				$this.data('editor', new Editor($this, settings));
			});
		};

	})(jQuery);
});
