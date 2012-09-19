$(function() {
	(function ($) {

		function Editor($el, settings) {
			this.$el = $el;
			$.extend(this, settings);

			var editor = this;

			this.setUp();
		}

		Editor.prototype.start = function () {
			this.$el.show();

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
					var $post = editor.$el.clone();
					$post.attr('id', '');
					$post.find('.body').html(data['Html']);
					var $permalink = $post.find('.time a');
					var posted = new Date(Date.parse(data['Posted']));
					$permalink.attr('href', '/' + posted.getUTCFullYear() + '/' + data['Id']);
					var postedText = $.relatizeDate.strftime(posted, "%i:%M <small>%p</small> %D %b %Y");
					$permalink.html(postedText);
					$('#newpost').after($post);
					$post.show();
				},
				error: function (xhr, textStatus, errorThrown) {
					alert('ERROR: ' + xhr.responseText);
				}
			});
			return false;
		};

		Editor.prototype.reset = function () {
			var $body = $('#newpost .body');
			$body.blur();
			$body.text('new post');

			$('#newpost').hide();

			$(document).bind('keypress.p', this.start.bind(this));
			return false;
		};

		Editor.prototype.setUp = function () {
			var $body = this.$el.find('.body');
			$body.bind('keydown.return', this.submit.bind(this));
			$body.bind('keydown.esc', this.reset.bind(this));

			$(document).bind('keypress.p', this.start.bind(this));
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
